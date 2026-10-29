package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/utils"
	"golang.org/x/time/rate"
)

// exploreLimiter paces calls to Fansly's content-discovery endpoints. They
// throttle aggressively and, when throttled, answer with an empty 200 body
// instead of a 429, so the collector below treats empty batches as transient.
var exploreLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)

// ExploreTag is a resolved hashtag from the content-discovery index.
type ExploreTag struct {
	ID        string `json:"id"`
	Tag       string `json:"tag"`
	PostCount int    `json:"postCount"`
	ViewCount int    `json:"viewCount"`
}

// SuggestionTag is a hashtag attached to an Explore suggestion.
type SuggestionTag struct {
	ID  string `json:"id"`
	Tag string `json:"tag"`
}

// MediaOfferSuggestion is a single entry from the Explore feed. Its AccountID
// is the requesting (viewer) account, not the creator; the creator is only
// known after resolving MediaOfferID into an AccountMedia (see GetMediaByIDs),
// whose AccountId field identifies the content owner.
type MediaOfferSuggestion struct {
	ID                 string          `json:"id"`
	AccountID          string          `json:"accountId"`
	MediaOfferID       string          `json:"mediaOfferId"`
	MediaOfferBundleID string          `json:"mediaOfferBundleId"`
	MediaID            string          `json:"mediaId"`
	PreviewID          string          `json:"previewId"`
	MediaType          int             `json:"mediaType"`
	Tags               []SuggestionTag `json:"tags"`
	PostTags           []SuggestionTag `json:"postTags"`
}

// hasAnyTag reports whether the suggestion carries one of the given tag IDs.
func (s MediaOfferSuggestion) hasAnyTag(tagIDs []string) bool {
	matches := func(t SuggestionTag) bool { return slices.Contains(tagIDs, t.ID) }
	return slices.ContainsFunc(s.PostTags, matches) || slices.ContainsFunc(s.Tags, matches)
}

type tagResolveResponse struct {
	Success  bool `json:"success"`
	Response struct {
		MediaOfferSuggestionTag *ExploreTag `json:"mediaOfferSuggestionTag"`
	} `json:"response"`
}

type suggestionsResponse struct {
	Success  bool `json:"success"`
	Response struct {
		MediaOfferSuggestions []MediaOfferSuggestion `json:"mediaOfferSuggestions"`
	} `json:"response"`
}

// ResolveExploreTag turns a human tag such as "pregnant" (with or without a
// leading '#') into its numeric content-discovery tag ID.
func ResolveExploreTag(ctx context.Context, tag string, fanslyHeaders *headers.FanslyHeaders) (*ExploreTag, error) {
	tag = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
	if tag == "" {
		return nil, fmt.Errorf("empty tag")
	}

	if err := exploreLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	reqURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/contentdiscovery/media/tag?tag=%s&ngsw-bypass=true", url.QueryEscape(tag))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tag resolve failed with status code %d", resp.StatusCode)
	}

	var decoded tagResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if decoded.Response.MediaOfferSuggestionTag == nil || decoded.Response.MediaOfferSuggestionTag.ID == "" {
		return nil, fmt.Errorf("tag %q not found", tag)
	}
	return decoded.Response.MediaOfferSuggestionTag, nil
}

// GetTagSuggestionsBatch fetches one page of the tag's Explore feed, the same
// request the web app makes on /explore/tag/<tag>. after is the ID of the last
// suggestion from the previous page ("0" for the first page). An empty result
// usually means the request was throttled, not that the feed has ended.
func GetTagSuggestionsBatch(ctx context.Context, tagIDs []string, after string, limit int, fanslyHeaders *headers.FanslyHeaders) ([]MediaOfferSuggestion, error) {
	if len(tagIDs) == 0 {
		return nil, fmt.Errorf("no tag IDs provided")
	}
	if limit <= 0 {
		limit = 25
	}
	if after == "" {
		after = "0"
	}

	if err := exploreLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	reqURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/contentdiscovery/media/suggestionsnew?before=0&after=%s&tagIds=%s&limit=%d&offset=0&ngsw-bypass=true",
		url.QueryEscape(after), strings.Join(tagIDs, ","), limit)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("explore suggestions failed with status code %d", resp.StatusCode)
	}

	var decoded suggestionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded.Response.MediaOfferSuggestions, nil
}

// CollectTagOffers pages through the Explore feed for the given tag IDs,
// accumulating distinct media offers until it reaches maxOffers or the feed
// stops yielding new items. maxOffers <= 0 collects until the feed is drained.
// progress, when non-nil, is called with the running count of distinct offers.
//
// Fansly throttles this feed by answering with empty pages, sometimes for
// several minutes at a time, and a throttled page looks the same as the end of
// the feed. An empty page is therefore retried until emptyPatience has passed
// before the feed is treated as finished.
func CollectTagOffers(ctx context.Context, tagIDs []string, maxOffers int, fanslyHeaders *headers.FanslyHeaders, progress func(found int)) ([]MediaOfferSuggestion, error) {
	const (
		pageSize       = 25
		emptyRetryWait = 15 * time.Second // pause before retrying an empty page
		emptyPatience  = 5 * time.Minute  // retry an empty page this long before giving up
		maxStalePages  = 3                // pages with nothing new before giving up
		safetyCap      = 1000             // hard ceiling so a deep feed can't loop forever
	)

	seen := make(map[string]bool)
	var collected []MediaOfferSuggestion
	after := "0"
	var emptySince time.Time
	stalePages := 0
	var lastErr error

	for len(collected) < safetyCap {
		if err := ctx.Err(); err != nil {
			return collected, err
		}

		batch, err := GetTagSuggestionsBatch(ctx, tagIDs, after, pageSize, fanslyHeaders)
		if err != nil {
			lastErr = err
			logger.Logger.Printf("[WARN] Explore batch failed: %v", err)
		}
		if len(batch) == 0 {
			if emptySince.IsZero() {
				emptySince = time.Now()
			}
			if time.Since(emptySince) >= emptyPatience {
				logger.Logger.Printf("[INFO] Explore: no data for %s, treating the feed as finished", emptyPatience)
				break
			}
			logger.Logger.Printf("[INFO] Explore: empty page (likely throttled), retrying in %s", emptyRetryWait)
			select {
			case <-ctx.Done():
				return collected, ctx.Err()
			case <-time.After(emptyRetryWait):
			}
			continue
		}
		emptySince = time.Time{}
		after = batch[len(batch)-1].ID

		added := 0
		for _, s := range batch {
			// The tag feed is filtered server-side; this guards against
			// unrelated recommendations ever reaching the download step.
			if !s.hasAnyTag(tagIDs) {
				continue
			}
			key := s.MediaOfferID
			if key == "" {
				key = s.ID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			collected = append(collected, s)
			added++
			if maxOffers > 0 && len(collected) >= maxOffers {
				if progress != nil {
					progress(len(collected))
				}
				return collected[:maxOffers], nil
			}
		}

		if added == 0 {
			// Pages with nothing new count toward giving up, so a feed that
			// repeats itself or stops matching the tag can't loop forever.
			stalePages++
			if stalePages >= maxStalePages {
				break
			}
			continue
		}
		stalePages = 0
		if progress != nil {
			progress(len(collected))
		}
	}

	if len(collected) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return collected, nil
}
