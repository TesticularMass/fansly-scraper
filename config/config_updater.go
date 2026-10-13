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
	def := CreateDefaultConfig()
	res := *existing

	// Overlay struct non-zero values and preserve defaults where zero
	mergeStruct(&res.Account, new.Account, def.Account)
	mergeStruct(&res.Options, new.Options, def.Options)
	mergeStruct(&res.LiveSettings, new.LiveSettings, def.LiveSettings)
	mergeStruct(&res.Notifications, new.Notifications, def.Notifications)
	res.SecurityHeaders = new.SecurityHeaders

	return &res
}

func mergeStruct[T any](dst *T, src T, fallback T) {
	dVal := reflect.ValueOf(dst).Elem()
	sVal := reflect.ValueOf(src)
	fVal := reflect.ValueOf(fallback)

	for i := 0; i < sVal.NumField(); i++ {
		sField := sVal.Field(i)
		dField := dVal.Field(i)
		fField := fVal.Field(i)

		switch sField.Kind() {
		case reflect.String:
			if sField.String() != "" {
				dField.SetString(sField.String())
			} else if dField.String() == "" {
				dField.SetString(fField.String())
			}
		case reflect.Int, reflect.Int64:
			if sField.Int() != 0 {
				dField.SetInt(sField.Int())
			} else if dField.Int() == 0 {
				dField.SetInt(fField.Int())
			}
		case reflect.Bool:
			dField.SetBool(sField.Bool())
		}
	}
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
