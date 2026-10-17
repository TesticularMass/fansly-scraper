package updater

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agnosto/fansly-scraper/utils"
)

type testRoundTripper struct {
	status int
	body   string
}

func (rt testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: rt.status,
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

func TestCheckUpdateAvailableNon200(t *testing.T) {
	old := utils.HTTPClient.Transport
	defer func() { utils.HTTPClient.Transport = old }()

	// Simulate 403 Forbidden (e.g. rate limit)
	utils.HTTPClient.Transport = testRoundTripper{
		status: http.StatusForbidden,
		body:   `{"message":"API rate limit exceeded"}`,
	}

	avail, _, err := CheckUpdateAvailable("v0.9.5")
	if err == nil {
		t.Errorf("expected error on HTTP 403, got nil (avail=%v)", avail)
	}
	if avail {
		t.Errorf("expected avail=false on HTTP 403 error")
	}
}
