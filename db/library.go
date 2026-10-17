package db

import (
	"fmt"

	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/utils"
	"gorm.io/gorm/clause"
)

func (d *Database) SaveFile(model, hash, path, fileType, postID string) error {
	file := &models.File{Model: model, Hash: hash, Path: path, FileType: fileType, PostID: postID}
	return d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash"}},
		DoNothing: true,
	}).Create(file).Error
}

func (d *Database) FileExists(path string) bool {
	var count int64
	err := d.DB.Model(&models.File{}).Where("path = ?", path).Count(&count).Error
	if err != nil {
		logger.Logger.Printf("Error checking if file exists: %v", err)
		return false
	}
	return count > 0
}

func (d *Database) MarkPostAsProcessed(postID, modelUsername, content string, postCreatedAt int64) error {
	post := &models.ProcessedPost{
		PostID: postID, ModelUsername: modelUsername, Content: content,
		Link:          fmt.Sprintf("https://fansly.com/post/%s", postID),
		PostCreatedAt: utils.ParseUnixTimestamp(postCreatedAt),
	}
	return d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "post_id"}},
		UpdateAll: true,
	}).Create(post).Error
}

func (d *Database) PostExists(postID string) bool {
	var count int64
	err := d.DB.Model(&models.ProcessedPost{}).Where("post_id = ?", postID).Count(&count).Error
	if err != nil {
		logger.Logger.Printf("Error checking if post exists: %v", err)
		return false
	}
	return count > 0
}
