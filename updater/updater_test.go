package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

func TestFailedUpdateKeepsExistingTmpUpdateDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	existing := filepath.Join("tmp_update", "keep.txt")
	if err := os.MkdirAll("tmp_update", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("user file"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := utils.HTTPClient.Transport
	defer func() { utils.HTTPClient.Transport = old }()
	utils.HTTPClient.Transport = testRoundTripper{status: http.StatusOK, body: "not a gzip archive"}

	var release GithubRelease
	asset := fmt.Sprintf("fansly-scraper_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if err := json.Unmarshal([]byte(fmt.Sprintf(`{"tag_name":"v1.0.0","assets":[{"name":%q,"browser_download_url":"https://example.invalid/update"}]}`, asset)), &release); err != nil {
		t.Fatal(err)
	}
	if err := updateBinary(&release); err == nil {
		t.Fatal("expected an invalid archive error")
	}
	if _, err := os.Stat(existing); err != nil {
		t.Fatalf("failed update removed an existing file: %v", err)
	}
}
