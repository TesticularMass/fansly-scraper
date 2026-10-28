package posts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// MediaOfferSuggestion is a single entry from the Explore feed. Its AccountID
// is the requesting (viewer) account, not the creator; the creator is only
// known after resolving MediaOfferID into an AccountMedia (see GetMediaByIDs),
// whose AccountId field identifies the content owner.
type MediaOfferSuggestion struct {
	ID                 string `json:"id"`
	AccountID          string `json:"accountId"`
	MediaOfferID       string `json:"mediaOfferId"`
	MediaOfferBundleID string `json:"mediaOfferBundleId"`
	MediaID            string `json:"mediaId"`
	PreviewID          string `json:"previewId"`
	MediaType          int    `json:"mediaType"`
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

// GetTagSuggestionsBatch fetches one rotating batch of Explore suggestions for
// the given tag IDs. The feed de-duplicates server-side, so repeated calls
// return different (and eventually empty) batches.
func GetTagSuggestionsBatch(ctx context.Context, tagIDs []string, limit int, fanslyHeaders *headers.FanslyHeaders) ([]MediaOfferSuggestion, error) {
	if len(tagIDs) == 0 {
		return nil, fmt.Errorf("no tag IDs provided")
	}
	if limit <= 0 {
		limit = 25
	}

	if err := exploreLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	reqURL := fmt.Sprintf("https://apiv3.fansly.com/api/v1/contentdiscovery/media/suggestionsnew?tagIds=%s&limit=%d&ngsw-bypass=true",
		strings.Join(tagIDs, ","), limit)
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

// CollectTagOffers polls the rotating Explore feed for the given tag IDs,
// accumulating distinct media offers until it reaches maxOffers or the feed
// stops yielding new items. maxOffers <= 0 collects until the feed is drained.
// progress, when non-nil, is called with the running count of distinct offers.
func CollectTagOffers(ctx context.Context, tagIDs []string, maxOffers int, fanslyHeaders *headers.FanslyHeaders, progress func(found int)) ([]MediaOfferSuggestion, error) {
	const (
		maxEmptyRounds = 3    // consecutive rounds with no new offers before stopping
		safetyCap      = 1000 // hard ceiling so a deep feed can't loop forever
	)

	seen := make(map[string]bool)
	var collected []MediaOfferSuggestion
	emptyRounds := 0
	var lastErr error

	for {
		if err := ctx.Err(); err != nil {
			return collected, err
		}

		batch, err := GetTagSuggestionsBatch(ctx, tagIDs, 25, fanslyHeaders)
		if err != nil {
			lastErr = err
			logger.Logger.Printf("[WARN] Explore batch failed: %v", err)
			emptyRounds++
			if emptyRounds >= maxEmptyRounds {
				break
			}
			continue
		}

		added := 0
		for _, s := range batch {
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
			emptyRounds++
		} else {
			emptyRounds = 0
			if progress != nil {
				progress(len(collected))
			}
		}

		if emptyRounds >= maxEmptyRounds || len(collected) >= safetyCap {
			break
		}
	}

	if len(collected) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return collected, nil
}
