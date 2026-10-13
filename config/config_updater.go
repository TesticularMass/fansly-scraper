package config

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
)

// VerifyConfigOnStartup runs all checks to ensure a valid config file exists and is updated.
// This should be called when the application starts.
func VerifyConfigOnStartup() {
	configPath := GetConfigPath()
	if err := EnsureConfigExists(configPath); err != nil {
		log.Printf("Error ensuring config exists: %v", err)
	}
	if _, err := LoadConfig(configPath); err == nil {
		if err := EnsureConfigUpdated(configPath); err != nil {
			log.Printf("Error updating config: %v", err)
		}
	}
}

// EnsureConfigExists checks if a config file is present. If not, it attempts to create one
// by copying an example, creating a default, or downloading it from the repo.
func EnsureConfigExists(configPath string) error {
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config doesn't exist, check for local example-config.toml
		if _, err := os.Stat("example-config.toml"); err == nil {
			err = copyFile("example-config.toml", configPath)
			if err == nil {
				return nil // Successfully copied example config
			}
			log.Printf("Failed to copy example config: %v. Trying next method.", err)
		}

		// Try to create a default config
		defaultConfig := CreateDefaultConfig()
		return SaveConfig(defaultConfig)
	}
	return nil
}

// EnsureConfigUpdated checks if the config file has all the latest fields and updates it with defaults if needed.
func EnsureConfigUpdated(configPath string) error {
	var rawConfig map[string]any
	metadata, err := toml.DecodeFile(configPath, &rawConfig)
	if err != nil {
		return err
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	defaults := CreateDefaultConfig()
	updated := false

	if reflect.DeepEqual(cfg.Options, OptionsConfig{}) {
		cfg.Options = defaults.Options
		updated = true
	} else {
		if cfg.Options.SaveLocation == "" {
			cfg.Options.SaveLocation = defaults.Options.SaveLocation
			updated = true
		}
		if !metadata.IsDefined("options", "skip_previews") {
			cfg.Options.SkipPreviews = defaults.Options.SkipPreviews
			updated = true
		}
		if !metadata.IsDefined("options", "use_content_as_filename") {
			cfg.Options.UseContentAsFilename = defaults.Options.UseContentAsFilename
			updated = true
		}
		if !metadata.IsDefined("options", "content_filename_template") {
			cfg.Options.ContentFilenameTemplate = defaults.Options.ContentFilenameTemplate
			updated = true
		}
		if !metadata.IsDefined("options", "download_media_type") {
			cfg.Options.DownloadMediaType = defaults.Options.DownloadMediaType
			updated = true
		}
		if !metadata.IsDefined("options", "skip_downloaded_posts") {
			cfg.Options.SkipDownloadedPosts = defaults.Options.SkipDownloadedPosts
			updated = true
		}
		if !metadata.IsDefined("options", "content_filename_length") {
			cfg.Options.ContentFilenameLength = defaults.Options.ContentFilenameLength
			updated = true
		}
		if !metadata.IsDefined("options", "date_format") {
			cfg.Options.DateFormat = defaults.Options.DateFormat
			updated = true
		}
		if !metadata.IsDefined("options", "download_profile_pic") {
			cfg.Options.DownloadProfilePic = defaults.Options.DownloadProfilePic
			updated = true
		}
		if !metadata.IsDefined("options", "post_limit") {
			cfg.Options.PostLimit = defaults.Options.PostLimit
			updated = true
		}
		if !metadata.IsDefined("options", "skip_long_videos") {
			cfg.Options.SkipLongVideos = defaults.Options.SkipLongVideos
			updated = true
		}
		if !metadata.IsDefined("options", "max_video_duration_seconds") {
			cfg.Options.MaxVideoDurationSeconds = defaults.Options.MaxVideoDurationSeconds
			updated = true
		}
		if !metadata.IsDefined("options", "apply_file_timestamps") {
			cfg.Options.ApplyFileTimestamps = defaults.Options.ApplyFileTimestamps
			updated = true
		}
	}

	if reflect.DeepEqual(cfg.Notifications, NotificationsConfig{}) {
		cfg.Notifications = defaults.Notifications
		updated = true
	} else {
		if !metadata.IsDefined("notifications", "notify_on_live_start") {
			cfg.Notifications.NotifyOnLiveStart = defaults.Notifications.NotifyOnLiveStart
			updated = true
		}
		if !metadata.IsDefined("notifications", "notify_on_live_end") {
			cfg.Notifications.NotifyOnLiveEnd = defaults.Notifications.NotifyOnLiveEnd
			updated = true
		}
		if !metadata.IsDefined("notifications", "send_contact_sheet_on_live_end") {
			cfg.Notifications.SendContactSheetOnLiveEnd = defaults.Notifications.SendContactSheetOnLiveEnd
			updated = true
		}
	}

	if reflect.DeepEqual(cfg.LiveSettings, LiveSettingsConfig{}) {
		cfg.LiveSettings = defaults.LiveSettings
		updated = true
	} else {
		if cfg.LiveSettings.VODsFileExtension == "" {
			cfg.LiveSettings.VODsFileExtension = defaults.LiveSettings.VODsFileExtension
			updated = true
		}
		if cfg.LiveSettings.FilenameTemplate == "" {
			cfg.LiveSettings.FilenameTemplate = defaults.LiveSettings.FilenameTemplate
			updated = true
		}
		if cfg.LiveSettings.DateFormat == "" {
			cfg.LiveSettings.DateFormat = defaults.LiveSettings.DateFormat
			updated = true
		}
		if !metadata.IsDefined("live_settings", "record_chat") {
			cfg.LiveSettings.RecordChat = defaults.LiveSettings.RecordChat
			updated = true
		}
		if !metadata.IsDefined("live_settings", "ffmpeg_recording_options") {
			cfg.LiveSettings.FFmpegRecordingOptions = defaults.LiveSettings.FFmpegRecordingOptions
			updated = true
		}
		if !metadata.IsDefined("live_settings", "ffmpeg_conversion_options") {
			cfg.LiveSettings.FFmpegConversionOptions = defaults.LiveSettings.FFmpegConversionOptions
			updated = true
		}
		if !metadata.IsDefined("live_settings", "check_interval") {
			cfg.LiveSettings.CheckInterval = defaults.LiveSettings.CheckInterval
			updated = true
		}
	}

	if !updated {
		return nil
	}
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(cfg)
}

// MergeConfigs merges a new configuration into an existing one.
// It prioritizes values from the new config but fills in missing/default values from the existing and default configs.
func MergeConfigs(existing, new *Config) *Config {
	result := &Config{}
	defaultConfig := CreateDefaultConfig()

	// Merge Account: Always take new values if they are not empty.
	result.Account = existing.Account
	if new.Account.AuthToken != "" {
		result.Account.AuthToken = new.Account.AuthToken
	}
	if new.Account.UserAgent != "" {
		result.Account.UserAgent = new.Account.UserAgent
	}

	// Merge Options
	result.Options = existing.Options
	if new.Options.SaveLocation != "" {
		result.Options.SaveLocation = new.Options.SaveLocation
	}
	result.Options.CheckUpdates = new.Options.CheckUpdates
	result.Options.M3U8Download = new.Options.M3U8Download
	result.Options.SkipPreviews = new.Options.SkipPreviews
	result.Options.UseContentAsFilename = new.Options.UseContentAsFilename
	if new.Options.ContentFilenameTemplate != "" {
		result.Options.ContentFilenameTemplate = new.Options.ContentFilenameTemplate
	} else if result.Options.ContentFilenameTemplate == "" {
		result.Options.ContentFilenameTemplate = defaultConfig.Options.ContentFilenameTemplate
	}
	if new.Options.DownloadMediaType != "" {
		result.Options.DownloadMediaType = new.Options.DownloadMediaType
	} else if result.Options.DownloadMediaType == "" {
		result.Options.DownloadMediaType = defaultConfig.Options.DownloadMediaType
	}
	result.Options.SkipDownloadedPosts = new.Options.SkipDownloadedPosts
	result.Options.DownloadProfilePic = new.Options.DownloadProfilePic

	if new.Options.PostLimit != 0 {
		result.Options.PostLimit = new.Options.PostLimit
	} else {
		result.Options.PostLimit = existing.Options.PostLimit
	}

	result.Options.SkipLongVideos = new.Options.SkipLongVideos
	if new.Options.MaxVideoDurationSeconds != 0 {
		result.Options.MaxVideoDurationSeconds = new.Options.MaxVideoDurationSeconds
	} else {
		result.Options.MaxVideoDurationSeconds = existing.Options.MaxVideoDurationSeconds
	}
	result.Options.ApplyFileTimestamps = new.Options.ApplyFileTimestamps

	// Merge LiveSettings
	result.LiveSettings = existing.LiveSettings
	if new.LiveSettings.SaveLocation != "" {
		result.LiveSettings.SaveLocation = new.LiveSettings.SaveLocation
	}
	if new.LiveSettings.VODsFileExtension != "" {
		result.LiveSettings.VODsFileExtension = new.LiveSettings.VODsFileExtension
	} else if result.LiveSettings.VODsFileExtension == "" {
		result.LiveSettings.VODsFileExtension = defaultConfig.LiveSettings.VODsFileExtension
	}
	if new.LiveSettings.FilenameTemplate != "" {
		result.LiveSettings.FilenameTemplate = new.LiveSettings.FilenameTemplate
	} else if result.LiveSettings.FilenameTemplate == "" {
		result.LiveSettings.FilenameTemplate = defaultConfig.LiveSettings.FilenameTemplate
	}
	if new.LiveSettings.DateFormat != "" {
		result.LiveSettings.DateFormat = new.LiveSettings.DateFormat
	} else if result.LiveSettings.DateFormat == "" {
		result.LiveSettings.DateFormat = defaultConfig.LiveSettings.DateFormat
	}

	// For booleans, we can just assign the value from the new config,
	// as it represents the most recent state.
	result.LiveSettings.FFmpegConvert = new.LiveSettings.FFmpegConvert
	result.LiveSettings.GenerateContactSheet = new.LiveSettings.GenerateContactSheet
	result.LiveSettings.UseMTForContactSheet = new.LiveSettings.UseMTForContactSheet
	result.LiveSettings.RecordChat = new.LiveSettings.RecordChat

	if new.LiveSettings.FFmpegRecordingOptions != "" {
		result.LiveSettings.FFmpegRecordingOptions = new.LiveSettings.FFmpegRecordingOptions
	}
	if new.LiveSettings.FFmpegConversionOptions != "" {
		result.LiveSettings.FFmpegConversionOptions = new.LiveSettings.FFmpegConversionOptions
	}

	if new.LiveSettings.CheckInterval > 0 {
		result.LiveSettings.CheckInterval = new.LiveSettings.CheckInterval
	} else {
		// If not set in new (0), use existing or default
		if existing.LiveSettings.CheckInterval > 0 {
			result.LiveSettings.CheckInterval = existing.LiveSettings.CheckInterval
		} else {
			result.LiveSettings.CheckInterval = 60
		}
	}

	// Merge Notifications
	result.Notifications = existing.Notifications
	result.Notifications.Enabled = new.Notifications.Enabled
	result.Notifications.SystemNotify = new.Notifications.SystemNotify
	result.Notifications.NotifyOnLiveStart = new.Notifications.NotifyOnLiveStart
	result.Notifications.NotifyOnLiveEnd = new.Notifications.NotifyOnLiveEnd
	if new.Notifications.DiscordWebhook != "" {
		result.Notifications.DiscordWebhook = new.Notifications.DiscordWebhook
	}
	if new.Notifications.DiscordMentionID != "" {
		result.Notifications.DiscordMentionID = new.Notifications.DiscordMentionID
	}
	if new.Notifications.TelegramBotToken != "" {
		result.Notifications.TelegramBotToken = new.Notifications.TelegramBotToken
	}
	if new.Notifications.TelegramChatID != "" {
		result.Notifications.TelegramChatID = new.Notifications.TelegramChatID
	}

	// Security Headers are always taken from the new config.
	result.SecurityHeaders = new.SecurityHeaders

	return result
}

// copyFile is a simple utility to copy a file from a source to a destination.
func copyFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ResetConfig removes the current config file (if present) and writes a fresh
// default config. This does not preserve any previous values.
func ResetConfig() error {
	configPath := GetConfigPath()
	// Best-effort remove existing file
	_ = os.Remove(configPath)
	// Ensure directory exists without creating a config file
	if err := os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
		return err
	}
	// Overwrite with a pristine default config
	defaultConfig := CreateDefaultConfig()
	// SaveConfig will not merge since we removed the file
	return SaveConfig(defaultConfig)
}
