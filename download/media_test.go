package download

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/db"
	"github.com/agnosto/fansly-scraper/posts"
	"github.com/schollz/progressbar/v3"
)

func TestDownloadSelectsHighestCompatibleVariant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.URL.Path)
	}))
	defer server.Close()
	config.SetConfigPath(filepath.Join(t.TempDir(), "config.toml"))
	defer config.SetConfigPath("")
	database, err := db.NewDatabase(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	for _, tt := range []struct {
		name         string
		m3u8, ffmpeg bool
	}{
		{"HLS disabled", false, true},
		{"ffmpeg unavailable", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var item posts.MediaItem
			body := `{"id":"main","mimetype":"video/mp4","height":480,"locations":[{"location":"SERVER/main.mp4"}],"variants":[
				{"id":"image","mimetype":"image/jpeg","height":4000,"locations":[{"location":"SERVER/image.jpg"}]},
				{"id":"missing","mimetype":"video/mp4","height":2160},
				{"id":"hls","mimetype":"application/vnd.apple.mpegurl","height":2160,"locations":[{"location":"SERVER/hls.m3u8"}]},
				{"id":"best","mimetype":"video/mp4","height":1080,"createdAt":1700000000123,"locations":[{"location":"SERVER/best.mp4"}]},
				{"id":"tie","mimetype":"video/mp4","height":1080,"locations":[{"location":"SERVER/tie.mp4"}]}
			]}`
			if err := json.Unmarshal([]byte(strings.ReplaceAll(body, "SERVER", server.URL)), &item); err != nil {
				t.Fatal(err)
			}
			cfg := config.CreateDefaultConfig()
			cfg.Options.ApplyFileTimestamps = true
			d := &Downloader{cfg: cfg, database: database, M3U8Download: tt.m3u8, ffmpegAvailable: tt.ffmpeg,
				progressBar: progressbar.NewOptions(-1, progressbar.OptionSetWriter(io.Discard))}
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "videos"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := d.DownloadMediaItem(context.Background(), posts.AccountMedia{Media: item}, dir, "creator", nil, 0); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "videos", "best.mp4")
			content, err := os.ReadFile(path)
			if err != nil || string(content) != "/best.mp4" {
				t.Fatalf("download = %q, error = %v", content, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.ModTime().Unix(); got != 1700000000 {
				t.Errorf("variant timestamp = %d, want 1700000000", got)
			}
		})
	}
}

func TestDownloadPreviewPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "preview")
	}))
	defer server.Close()
	config.SetConfigPath(filepath.Join(t.TempDir(), "config.toml"))
	defer config.SetConfigPath("")
	database, err := db.NewDatabase(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, tt := range []struct {
		name              string
		skip, force, want bool
	}{
		{"enabled", false, false, true},
		{"skipped", true, false, false},
		{"explore override", true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.CreateDefaultConfig()
			cfg.Options.SkipPreviews = tt.skip
			d := &Downloader{cfg: cfg, database: database, progressBar: progressbar.NewOptions(-1, progressbar.OptionSetWriter(io.Discard))}
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "images"), 0755); err != nil {
				t.Fatal(err)
			}
			media := posts.AccountMedia{Preview: &posts.MediaItem{
				ID: "preview", Mimetype: "image/jpeg", Locations: []posts.Location{{Location: server.URL + "/preview.jpg"}},
			}}
			if err := d.downloadMediaItem(context.Background(), media, dir, "creator", nil, 0, tt.force, true); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(dir, "images", "preview_preview.jpg"))
			if tt.want {
				if err != nil || string(content) != "preview" {
					t.Fatalf("preview = %q, error = %v", content, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("skipped preview exists or could not be checked: %v", err)
			}
		})
	}
}
