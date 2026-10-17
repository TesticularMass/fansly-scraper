package db

import (
	"path/filepath"
	"testing"

	"github.com/agnosto/fansly-scraper/db/models"
)

func TestLibraryOperations(t *testing.T) {
	database, err := openDB(filepath.Join(t.TempDir(), "downloads.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if database.FileExists("/video.mp4") {
		t.Fatal("file unexpectedly exists")
	}
	if err := database.SaveFile("model", "hash", "/video.mp4", "video", "post"); err != nil {
		t.Fatal(err)
	}
	if err := database.SaveFile("model", "hash", "/duplicate.mp4", "video", "post"); err != nil {
		t.Fatal(err)
	}
	if !database.FileExists("/video.mp4") || database.FileExists("/duplicate.mp4") {
		t.Fatal("duplicate hash changed saved file")
	}

	if database.PostExists("post") {
		t.Fatal("post unexpectedly exists")
	}
	if err := database.MarkPostAsProcessed("post", "model", "old", 100); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkPostAsProcessed("post", "model", "new", 200); err != nil {
		t.Fatal(err)
	}
	var post models.ProcessedPost
	if err := database.DB.Where("post_id = ?", "post").First(&post).Error; err != nil {
		t.Fatal(err)
	}
	if !database.PostExists("post") || post.Content != "new" || post.PostCreatedAt.Unix() != 200 {
		t.Fatalf("post upsert failed: %+v", post)
	}

	// Test millisecond timestamps
	if err := database.MarkPostAsProcessed("post_ms", "model", "content", 1700000000000); err != nil {
		t.Fatal(err)
	}
	var postMS models.ProcessedPost
	if err := database.DB.Where("post_id = ?", "post_ms").First(&postMS).Error; err != nil {
		t.Fatal(err)
	}
	if postMS.PostCreatedAt.Unix() != 1700000000 {
		t.Fatalf("millisecond timestamp was not converted: got %v", postMS.PostCreatedAt.Unix())
	}
}
