package core

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/utils"
)

type StreamResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Stream struct {
			Status        int    `json:"status"`
			ViewerCount   int    `json:"viewerCount"`
			LastFetchedAt int64  `json:"lastFetchedAt"`
			PlaybackUrl   string `json:"playbackUrl"`
			Access        bool   `json:"access"`
		} `json:"stream"`
	} `json:"response"`
}

func CheckIfModelIsLive(modelID string) (bool, string, error) {
	fanslyHeaders, err := headers.GetCachedHeaders()
	if err != nil {
		return false, "", fmt.Errorf("error creating headers: %v", err)
	}

	url := fmt.Sprintf("https://apiv3.fansly.com/api/v1/streaming/channel/%s", modelID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %v", err)
	}

	// Use the headers package to add headers
	fanslyHeaders.AddHeadersToRequest(req, true)

	client := utils.HTTPClient
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("live check failed with status code %d", resp.StatusCode)
	}

	var streamResp StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		return false, "", fmt.Errorf("failed to decode response: %v", err)
	}

	isLive := streamResp.Success &&
		streamResp.Response.Stream.Status == 2 &&
		streamResp.Response.Stream.Access

	return isLive, streamResp.Response.Stream.PlaybackUrl, nil
}
