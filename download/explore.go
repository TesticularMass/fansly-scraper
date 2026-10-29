package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"

	"github.com/agnosto/fansly-scraper/auth"
	"github.com/agnosto/fansly-scraper/core"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
)

// exploreFilterScanWindow bounds how many offers a creator-filtered explore
// run will pull from the tag feed. A suggestion does not reveal its
// creator until its media is resolved, so the feed can't be filtered as it is
// read; scanning a bounded window keeps filtered runs from draining the whole
// (potentially large) feed.
const exploreFilterScanWindow = 200

// PostLimit returns the configured per-run item limit (0 means no limit).
func (d *Downloader) PostLimit() int {
	return d.cfg.Options.PostLimit
}

// DownloadExplore discovers premium-post previews from Fansly's Explore feed
// by tag and downloads their preview media (plus full media where the account
// already has access). When usernames is non-empty, only offers whose creator
// matches one of those usernames are kept. maxItems caps the number of media
// items downloaded; maxItems <= 0 drains the tag feed.
func (d *Downloader) DownloadExplore(ctx context.Context, tags []string, usernames []string, maxItems int) error {
	if _, err := auth.Login(d.headers); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Resolve tags -> content-discovery tag IDs.
	var tagIDs, tagLabels []string
	for _, t := range tags {
		et, err := posts.ResolveExploreTag(ctx, t, d.headers)
		if err != nil {
			logger.Logger.Printf("[WARN] Explore: could not resolve tag %q: %v", t, err)
			continue
		}
		tagIDs = append(tagIDs, et.ID)
		tagLabels = append(tagLabels, et.Tag)
		logger.Logger.Printf("[INFO] Explore: tag %q -> id=%s (%d posts)", et.Tag, et.ID, et.PostCount)
	}
	if len(tagIDs) == 0 {
		return fmt.Errorf("no valid tags resolved from %v", tags)
	}

	// Resolve the optional username filter into a set of creator account IDs.
	var filterSet map[string]bool
	if len(usernames) > 0 {
		filterSet = make(map[string]bool)
		for _, u := range usernames {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			id, err := core.GetModelIDFromUsername(u)
			if err != nil {
				logger.Logger.Printf("[WARN] Explore: could not resolve username %q: %v", u, err)
				continue
			}
			filterSet[id] = true
		}
		if len(filterSet) == 0 {
			return fmt.Errorf("none of the filter usernames could be resolved: %v", usernames)
		}
	}

	// Without a user filter we can stop once we have maxItems offers; with a
	// filter we must scan the whole tag feed since it is not filterable
	// server-side.
	collectCap := maxItems
	if filterSet != nil {
		// Scan a bounded window rather than draining the entire feed. Honour a
		// larger explicit -l if the user asked for more matches.
		collectCap = exploreFilterScanWindow
		if maxItems > collectCap {
			collectCap = maxItems
		}
		fmt.Printf("Scanning up to %d previews for matching creator(s)...\n", collectCap)
	}
	fmt.Printf("Exploring #%s (Fansly throttles this feed, so this can take a few minutes)...\n", strings.Join(tagLabels, ", #"))
	offers, err := posts.CollectTagOffers(ctx, tagIDs, collectCap, d.headers, func(found int) {
		logger.Logger.Printf("[INFO] Explore: collected %d offers so far", found)
		fmt.Printf("Found %d tagged previews so far...\n", found)
	})
	if err != nil && len(offers) == 0 {
		return fmt.Errorf("explore discovery failed: %w", err)
	}
	logger.Logger.Printf("[INFO] Explore: collected %d distinct offers", len(offers))

	// A suggestion's mediaOfferId IS the AccountMedia id; resolving it yields
	// the media + preview locations and the true creator account id.
	var offerIDs []string
	skippedBundles := 0
	for _, o := range offers {
		if o.MediaOfferID != "" {
			offerIDs = append(offerIDs, o.MediaOfferID)
		} else {
			skippedBundles++
		}
	}
	if skippedBundles > 0 {
		logger.Logger.Printf("[INFO] Explore: skipped %d bundle offers without a direct media id", skippedBundles)
	}
	if len(offerIDs) == 0 {
		fmt.Println("No downloadable explore previews found.")
		return nil
	}

	mediaItems, err := posts.GetMediaByIDs(ctx, offerIDs, d.headers)
	if err != nil {
		logger.Logger.Printf("[WARN] Explore: some media failed to resolve: %v", err)
	}

	// Filter to requested creators by the resolved media's account id.
	if filterSet != nil {
		kept := mediaItems[:0]
		for _, am := range mediaItems {
			if filterSet[am.AccountId] {
				kept = append(kept, am)
			}
		}
		mediaItems = kept
	}

	if maxItems > 0 && len(mediaItems) > maxItems {
		mediaItems = mediaItems[:maxItems]
	}

	if len(mediaItems) == 0 {
		if filterSet != nil {
			fmt.Println("No explore previews matched the requested user(s).")
		} else {
			fmt.Println("No downloadable explore previews found.")
		}
		return nil
	}

	// Group by creator and resolve usernames for folder naming.
	byAccount := make(map[string][]posts.AccountMedia)
	var accountIDs []string
	for _, am := range mediaItems {
		if _, ok := byAccount[am.AccountId]; !ok {
			accountIDs = append(accountIDs, am.AccountId)
		}
		byAccount[am.AccountId] = append(byAccount[am.AccountId], am)
	}

	nameByID := make(map[string]string, len(accountIDs))
	if details, derr := auth.GetAccountDetails(accountIDs, d.headers); derr != nil {
		logger.Logger.Printf("[WARN] Explore: failed to resolve some creator names: %v", derr)
	} else {
		for _, m := range details {
			nameByID[m.ID] = m.Username
		}
	}

	d.progressBar = progressbar.NewOptions(len(mediaItems),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan]Explore #%s[reset]", strings.Join(tagLabels, ",#"))),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer: "[green]=[reset]", SaucerHead: "[green]>[reset]", SaucerPadding: " ", BarStart: "[", BarEnd: "]",
		}),
	)

	for _, accountID := range accountIDs {
		if ctx.Err() != nil {
			break
		}
		username := nameByID[accountID]
		if username == "" {
			username = accountID
		}
		baseDir := filepath.Join(d.saveLocation, strings.ToLower(username), "explore")
		for _, sub := range []string{"images", "videos", "audios"} {
			if err := os.MkdirAll(filepath.Join(baseDir, sub), os.ModePerm); err != nil {
				logger.Logger.Printf("[ERROR] Explore: failed to create dir for %s: %v", username, err)
			}
		}

		for i, am := range byAccount[accountID] {
			if ctx.Err() != nil {
				break
			}
			src := posts.Post{ID: am.ID, CreatedAt: am.CreatedAt}
			if err := d.downloadMediaItem(ctx, am, baseDir, username, src, i, true, false); err != nil {
				logger.Logger.Printf("[ERROR] Explore: failed media %s for %s: %v", am.ID, username, err)
			}
			d.progressBar.Add(1)
		}
	}

	d.progressBar.Finish()
	fmt.Println()
	return nil
}
