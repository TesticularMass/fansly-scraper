package posts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/utils"
	"golang.org/x/time/rate"
)

var (
	limiter = rate.NewLimiter(rate.Every(5*time.Second), 1)
)

type AccountMediaBundle struct {
	ID              string   `json:"id"`
	Access          bool     `json:"access"`
	AccountMediaIDs []string `json:"accountMediaIds"`
	BundleContent   []struct {
		AccountMediaID string `json:"accountMediaId"`
		Pos            int    `json:"pos"`
	} `json:"bundleContent"`
}

// Attachment references media on a post or message: content type 1 is a
// single AccountMedia, 2 is an AccountMediaBundle.
type Attachment struct {
	ContentType int    `json:"contentType"`
	ContentID   string `json:"contentId"`
}

// ErrMissingMedia means a post references media the API did not return, so
// the post must not be marked processed.
var ErrMissingMedia = errors.New("post media incomplete")

type AccountMediaResponse struct {
	Success  bool           `json:"success"`
	Response []AccountMedia `json:"response"`
}

type Location struct {
	Location string            `json:"location"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type MediaItem struct {
	ID        string `json:"id"`
	Type      int    `json:"type"`
	Height    int    `json:"height"`
	Mimetype  string `json:"mimetype"`
	Metadata  string `json:"metadata,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	Variants  []struct {
		ID        string     `json:"id"`
		Type      int        `json:"type"`
		Height    int        `json:"height"`
		Mimetype  string     `json:"mimetype"`
		Metadata  string     `json:"metadata,omitempty"`
		CreatedAt int64      `json:"createdAt,omitempty"`
		UpdatedAt int64      `json:"updatedAt,omitempty"`
		Locations []Location `json:"locations"`
	} `json:"variants"`
	Locations []Location `json:"locations"`
}

type AccountMedia struct {
	ID        string     `json:"id"`
	AccountId string     `json:"accountId"`
	Access    bool       `json:"access"`
	CreatedAt int64      `json:"createdAt,omitempty"`
	Media     MediaItem  `json:"media"`
	Preview   *MediaItem `json:"preview,omitempty"`
}

// HasLocations reports whether the item or any variant has a download location.
func (m MediaItem) HasLocations() bool {
	if len(m.Locations) > 0 {
		return true
	}
	for _, variant := range m.Variants {
		if len(variant.Locations) > 0 {
			return true
		}
	}
	return false
}

// HasLocations includes preview-only content as well as the main media.
func (m AccountMedia) HasLocations() bool {
	return m.Media.HasLocations() || (m.Preview != nil && m.Preview.HasLocations())
}

// PostInfo struct to capture details from a single post API response
type PostInfo struct {
	ID          string       `json:"id"`
	AccountId   string       `json:"accountId"`
	Content     string       `json:"content"`
	CreatedAt   int64        `json:"createdAt"`
	Attachments []Attachment `json:"attachments"`
}

type PostResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Posts               []PostInfo           `json:"posts"`
		AccountMediaBundles []AccountMediaBundle `json:"accountMediaBundles"`
		AccountMedia        []AccountMedia       `json:"accountMedia"`
	} `json:"response"`
}

// GetFullPostDetails fetches post metadata and all associated media items.
// When some referenced media is unavailable it returns the post, the media it
// could fetch, and an error wrapping ErrMissingMedia.
func GetFullPostDetails(ctx context.Context, postId string, fanslyHeaders *headers.FanslyHeaders) (*PostInfo, []AccountMedia, error) {
	if err := limiter.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("rate limiter error: %w", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/post?ids=%s&ngsw-bypass=true", postId)
	logger.Logger.Printf("[INFO] Fetching details for Post: %s", postId)

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	fanslyHeaders.AddHeadersToRequest(req, true)

	client := utils.HTTPClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to fetch post with status code %d", resp.StatusCode)
	}

	var postResp PostResponse
	if err := json.NewDecoder(resp.Body).Decode(&postResp); err != nil {
		return nil, nil, err
	}
	if !postResp.Success || len(postResp.Response.Posts) == 0 {
		return nil, nil, fmt.Errorf("API request for post %s failed or returned no data", postId)
	}

	postInfo := postResp.Response.Posts[0]

	// 1. Map existing media and bundles for quick lookup
	mediaMap := make(map[string]AccountMedia)
	for _, media := range postResp.Response.AccountMedia {
		mediaMap[media.ID] = media
	}

	bundleMap := make(map[string]AccountMediaBundle)
	for _, bundle := range postResp.Response.AccountMediaBundles {
		bundleMap[bundle.ID] = bundle
	}

	// 2. Build an ORDERED list of Media IDs from attachments
	orderedIDs := orderedMediaIDs(postInfo.Attachments, bundleMap)

	// 3. Identify which IDs are missing from the initial response
	var mediaToFetch []string
	for _, id := range orderedIDs {
		if _, ok := mediaMap[id]; !ok {
			mediaToFetch = append(mediaToFetch, id)
		}
	}

	// 4. Fetch missing media
	if len(mediaToFetch) > 0 {
		fetchedMedia, err := GetMediaByIDs(ctx, mediaToFetch, fanslyHeaders)
		if err != nil {
			logger.Logger.Printf("[WARN] Failed to fetch some bundled media for post %s: %v", postId, err)
		}
		// Add fetched items to map
		for _, media := range fetchedMedia {
			mediaMap[media.ID] = media
		}
	}

	// 5. Construct final slice in strict order
	var finalMediaItems []AccountMedia
	for _, id := range orderedIDs {
		if media, ok := mediaMap[id]; ok {
			finalMediaItems = append(finalMediaItems, media)
		}
	}

	logger.Logger.Printf("[INFO] Retrieved %d media items for post %s", len(finalMediaItems), postId)
	if missing := len(orderedIDs) - len(finalMediaItems); missing > 0 {
		return &postInfo, finalMediaItems, fmt.Errorf("%w: %d of %d items unavailable for post %s", ErrMissingMedia, missing, len(orderedIDs), postId)
	}
	return &postInfo, finalMediaItems, nil
}

// orderedMediaIDs builds the ordered, de-duplicated list of AccountMedia IDs
// referenced by attachments. Bundle content is sorted by position; bundles
// without position data fall back to their plain ID list.
func orderedMediaIDs(attachments []Attachment, bundleMap map[string]AccountMediaBundle) []string {
	var orderedIDs []string
	seenIDs := make(map[string]bool)
	add := func(id string) {
		if !seenIDs[id] {
			seenIDs[id] = true
			orderedIDs = append(orderedIDs, id)
		}
	}

	for _, attachment := range attachments {
		switch attachment.ContentType {
		case 1:
			add(attachment.ContentID)
		case 2:
			bundle, ok := bundleMap[attachment.ContentID]
			if !ok {
				continue
			}
			if len(bundle.BundleContent) == 0 {
				for _, id := range bundle.AccountMediaIDs {
					add(id)
				}
				continue
			}
			// Sort a copy: the bundle shares its slice with the response.
			content := slices.Clone(bundle.BundleContent)
			sort.SliceStable(content, func(i, j int) bool { return content[i].Pos < content[j].Pos })
			for _, item := range content {
				add(item.AccountMediaID)
			}
		}
	}

	return orderedIDs
}

// GetPostsMediaBatch fetches media for many posts using batched /post?ids=
// calls. The per-post GetFullPostDetails path costs one rate-limited request
// per post; batching cuts timeline preparation time by the batch size.
// The result maps post ID to its ordered media; posts missing from the map
// failed to fetch and should not be marked processed.
func GetPostsMediaBatch(ctx context.Context, postIDs []string, fanslyHeaders *headers.FanslyHeaders) (map[string][]AccountMedia, error) {
	result := make(map[string][]AccountMedia, len(postIDs))
	if len(postIDs) == 0 {
		return result, nil
	}

	const batchSize = 25
	var firstErr error

	for i := 0; i < len(postIDs); i += batchSize {
		end := min(i+batchSize, len(postIDs))
		batch := postIDs[i:end]

		if err := limiter.Wait(ctx); err != nil {
			return result, fmt.Errorf("rate limiter error: %w", err)
		}

		url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/post?ids=%s&ngsw-bypass=true", strings.Join(batch, ","))
		logger.Logger.Printf("[INFO] Fetching details for %d posts (batch %d/%d)",
			len(batch), (i/batchSize)+1, (len(postIDs)+batchSize-1)/batchSize)

		postResp, err := fetchPostBatch(ctx, url, fanslyHeaders)
		if err != nil {
			logger.Logger.Printf("[WARN] Failed to fetch post batch: %v", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		mediaMap := make(map[string]AccountMedia)
		for _, media := range postResp.Response.AccountMedia {
			mediaMap[media.ID] = media
		}
		bundleMap := make(map[string]AccountMediaBundle)
		for _, bundle := range postResp.Response.AccountMediaBundles {
			bundleMap[bundle.ID] = bundle
		}

		orderedByPost := make(map[string][]string, len(postResp.Response.Posts))
		var missing []string
		for _, post := range postResp.Response.Posts {
			ids := orderedMediaIDs(post.Attachments, bundleMap)
			orderedByPost[post.ID] = ids
			for _, id := range ids {
				if _, ok := mediaMap[id]; !ok {
					missing = append(missing, id)
				}
			}
		}

		if len(missing) > 0 {
			fetchedMedia, err := GetMediaByIDs(ctx, missing, fanslyHeaders)
			if err != nil {
				logger.Logger.Printf("[WARN] Failed to fetch some bundled media for post batch: %v", err)
			}
			for _, media := range fetchedMedia {
				mediaMap[media.ID] = media
			}
		}

		for postID, ids := range orderedByPost {
			items := make([]AccountMedia, 0, len(ids))
			hasMissing := false
			for _, id := range ids {
				if media, ok := mediaMap[id]; ok {
					items = append(items, media)
				} else {
					hasMissing = true
				}
			}
			if hasMissing {
				logger.Logger.Printf("[WARN] Post %s is missing %d referenced media items, leaving for retry", postID, len(ids)-len(items))
				continue
			}
			result[postID] = items
		}
	}

	return result, firstErr
}

func fetchPostBatch(ctx context.Context, url string, fanslyHeaders *headers.FanslyHeaders) (*PostResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
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
		return nil, fmt.Errorf("post batch request failed with status code %d", resp.StatusCode)
	}

	var postResp PostResponse
	if err := json.NewDecoder(resp.Body).Decode(&postResp); err != nil {
		return nil, err
	}
	if !postResp.Success {
		return nil, fmt.Errorf("API reported failure for post batch")
	}
	return &postResp, nil
}

func GetMediaByIDs(ctx context.Context, mediaIDs []string, fanslyHeaders *headers.FanslyHeaders) ([]AccountMedia, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}

	// Create a set of unique IDs to avoid duplicate processing
	idSet := make(map[string]struct{})
	uniqueIDs := []string{}
	for _, id := range mediaIDs {
		if _, ok := idSet[id]; !ok {
			idSet[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	if len(uniqueIDs) == 0 {
		return nil, nil
	}

	var allFetchedMedia []AccountMedia
	const batchSize = 100
	var firstErr error

	for i := 0; i < len(uniqueIDs); i += batchSize {
		end := i + batchSize
		if end > len(uniqueIDs) {
			end = len(uniqueIDs)
		}
		batch := uniqueIDs[i:end]

		if err := limiter.Wait(ctx); err != nil {
			// If the context is cancelled (e.g., timeout), stop processing.
			if ctx.Err() != nil {
				return allFetchedMedia, fmt.Errorf("context cancelled during batch processing: %w", ctx.Err())
			}
			return allFetchedMedia, fmt.Errorf("rate limiter error: %w", err)
		}

		url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/account/media?ids=%s&ngsw-bypass=true", strings.Join(batch, ","))
		logger.Logger.Printf("[INFO] Fetching details for %d bundled media items (batch %d/%d)", len(batch), (i/batchSize)+1, (len(uniqueIDs)/batchSize)+1)

		func() {
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				logger.Logger.Printf("[WARN] Failed to create request for media batch: %v", err)
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			fanslyHeaders.AddHeadersToRequest(req, true)

			client := utils.HTTPClient
			resp, err := client.Do(req)
			if err != nil {
				logger.Logger.Printf("[WARN] Failed to execute request for media batch: %v", err)
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logger.Logger.Printf("[WARN] Failed to fetch media by IDs with status code %d for batch", resp.StatusCode)
				if firstErr == nil {
					firstErr = fmt.Errorf("status code %d", resp.StatusCode)
				}
				return
			}

			var mediaResp AccountMediaResponse
			if err := json.NewDecoder(resp.Body).Decode(&mediaResp); err != nil {
				logger.Logger.Printf("[WARN] Failed to decode media response for batch: %v", err)
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			if !mediaResp.Success {
				logger.Logger.Printf("[WARN] API reported failure when fetching media by IDs for batch")
				if firstErr == nil {
					firstErr = fmt.Errorf("API reported failure")
				}
				return
			}

			allFetchedMedia = append(allFetchedMedia, mediaResp.Response...)
		}()
	}

	if len(allFetchedMedia) != len(uniqueIDs) {
		logger.Logger.Printf("[WARN] Expected to fetch details for %d unique media IDs, but got %d. Some items may have been deleted or are inaccessible.", len(uniqueIDs), len(allFetchedMedia))
	}

	return allFetchedMedia, firstErr
}

func filterMediaWithLocations(mediaItems []AccountMedia) []AccountMedia {
	var filteredMedia []AccountMedia

	for _, accountMedia := range mediaItems {
		hasContent := accountMedia.HasLocations()

		// 3. Specifically check for streamable videos (HLS/DASH), as they may not have explicit `locations`.
		// The download logic knows how to handle these manifest types.
		if !hasContent && accountMedia.Media.Type == 2 { // Type 2 is video
			for _, variant := range accountMedia.Media.Variants {
				// Type 302 is HLS (m3u8), 303 is DASH (mpd)
				if variant.Type == 302 || variant.Type == 303 {
					hasContent = true
					break
				}
			}
		}

		if hasContent {
			filteredMedia = append(filteredMedia, accountMedia)
		} else {
			logger.Logger.Printf("[WARN] Skipping AccountMedia %s: No downloadable/streamable content found", accountMedia.ID)
		}
	}

	return filteredMedia
}
