package download

import (
	"testing"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/posts"
)

func TestGenerateFilenameWithDateTemplate(t *testing.T) {
	cfg := config.CreateDefaultConfig()
	cfg.Options.UseContentAsFilename = true
	cfg.Options.DateFormat = "2006-01-02"
	cfg.Options.ContentFilenameTemplate = "{date}_{model_name}_{postId}_{index}"

	d := &Downloader{cfg: cfg}

	post := posts.Post{
		ID:        "post999",
		Content:   "Hello World",
		CreatedAt: 1700000000000, // Fansly sends millisecond timestamps
	}
	media := posts.MediaItem{
		ID: "media111",
	}

	name := d.generateFilename(media, "TestCreator", post, 0, false, ".mp4")
	if len(name) < 10 || name[10] != '_' {
		t.Fatalf("unexpected filename format: %s", name)
	}

	// Test without content (empty caption) using default template
	cfg.Options.ContentFilenameTemplate = "{date}-{content}_{index}"
	postEmpty := posts.Post{
		ID:        "post999",
		Content:   "",
		CreatedAt: 1700000000000,
	}
	nameEmpty := d.generateFilename(media, "TestCreator", postEmpty, 0, false, ".mp4")
	if nameEmpty == "" || nameEmpty[0] != '2' {
		t.Fatalf("unexpected filename for empty content: %s", nameEmpty)
	}

	// Test that empty captions on the same day for different posts do not collide
	postEmpty2 := posts.Post{
		ID:        "post888",
		Content:   "",
		CreatedAt: 1700000000000,
	}
	nameEmpty2 := d.generateFilename(media, "TestCreator", postEmpty2, 0, false, ".mp4")
	if nameEmpty == nameEmpty2 {
		t.Fatalf("filenames collided for empty content across different posts: %s == %s", nameEmpty, nameEmpty2)
	}

	// Test unicode caption preservation
	postUnicode := posts.Post{
		ID:        "post777",
		Content:   "日本語",
		CreatedAt: 1700000000000,
	}
	nameUnicode := d.generateFilename(media, "TestCreator", postUnicode, 0, false, ".mp4")
	if nameUnicode == nameEmpty {
		t.Fatalf("unicode caption unexpectedly collided with empty content: %s", nameUnicode)
	}
}
