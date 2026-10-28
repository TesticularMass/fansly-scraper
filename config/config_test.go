package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"spaces to underscores", "my file name", "my_file_name"},
		{"colon to dash", "12:34:56", "12-34-56"},
		{"strips unsafe chars", "a<b>c?d*e|f", "abcdef"},
		{"keeps word chars and dots", "video_01.final-cut", "video_01.final-cut"},
		{"empty becomes placeholder", "", "empty_content"},
		{"only unsafe becomes placeholder", "<>?*|", "empty_content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFilename(tt.in); got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatVODFilename(t *testing.T) {
	data := map[string]string{
		"model_username": "some model",
		"streamId":       "123",
		"streamVersion":  "2",
	}
	got := FormatVODFilename("{model_username}_{streamId}_{streamVersion}", data)
	want := "some_model_123_v2"
	if got != want {
		t.Errorf("FormatVODFilename = %q, want %q", got, want)
	}
}

func TestDefaultConfigApplyFileTimestamps(t *testing.T) {
	cfg := CreateDefaultConfig()
	if cfg.Options.ApplyFileTimestamps != false {
		t.Errorf("expected ApplyFileTimestamps to default to false, got %v", cfg.Options.ApplyFileTimestamps)
	}
}

func TestEnsureConfigUpdatedAddMissingField(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Write an old config file without apply_file_timestamps
	oldConfig := `[account]
auth_token = "dummy_token"
user_agent = "dummy_agent"

[options]
save_location = "/tmp/downloads"
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err := EnsureConfigUpdated(configPath)
	if err != nil {
		t.Fatalf("EnsureConfigUpdated failed: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Options.ApplyFileTimestamps != false {
		t.Errorf("expected ApplyFileTimestamps to be updated to false, got %v", cfg.Options.ApplyFileTimestamps)
	}
}

func TestEnsureConfigUpdatedKeepsExplicitValues(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	oldConfig := `[account]
auth_token = "dummy_token"
user_agent = "dummy_agent"

[options]
save_location = "/tmp/downloads"
skip_previews = false

[live_settings]
vods_file_extension = ".ts"
record_chat = false

[notifications]
enabled = true
notify_on_live_start = false
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfigUpdated(configPath); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Options.SkipPreviews || !cfg.Options.DownloadProfilePic || cfg.LiveSettings.RecordChat || cfg.LiveSettings.CheckInterval != 60 || cfg.Notifications.NotifyOnLiveStart {
		t.Fatalf("skip_previews=%v download_profile_pic=%v record_chat=%v check_interval=%d notify_on_live_start=%v", cfg.Options.SkipPreviews, cfg.Options.DownloadProfilePic, cfg.LiveSettings.RecordChat, cfg.LiveSettings.CheckInterval, cfg.Notifications.NotifyOnLiveStart)
	}
}

func TestMergeConfigs(t *testing.T) {
	existing := CreateDefaultConfig()
	existing.Account.AuthToken = "old-token"
	existing.Account.UserAgent = "old-ua"
	existing.Options.SaveLocation = "/old/location"
	existing.Notifications.DiscordWebhook = "https://old.webhook"
	existing.LiveSettings.CheckInterval = 120
	existing.Options.ApplyFileTimestamps = false

	incoming := CreateDefaultConfig()
	incoming.Account.AuthToken = "new-token"
	incoming.Account.UserAgent = "" // Empty must not clobber existing
	incoming.Options.SaveLocation = ""
	incoming.Notifications.DiscordWebhook = ""
	incoming.LiveSettings.CheckInterval = 0 // Unset must fall back to existing
	incoming.Options.ApplyFileTimestamps = true

	merged := MergeConfigs(existing, incoming)

	if merged.Account.AuthToken != "new-token" {
		t.Errorf("AuthToken = %q, want new-token", merged.Account.AuthToken)
	}
	if merged.Account.UserAgent != "old-ua" {
		t.Errorf("UserAgent = %q, want old-ua (empty new value must not clobber)", merged.Account.UserAgent)
	}
	if merged.Options.SaveLocation != "/old/location" {
		t.Errorf("SaveLocation = %q, want /old/location", merged.Options.SaveLocation)
	}
	if merged.Notifications.DiscordWebhook != "https://old.webhook" {
		t.Errorf("DiscordWebhook = %q, want preserved old webhook", merged.Notifications.DiscordWebhook)
	}
	if merged.LiveSettings.CheckInterval != 120 {
		t.Errorf("CheckInterval = %d, want 120 (0 must fall back to existing)", merged.LiveSettings.CheckInterval)
	}
	if merged.Options.ApplyFileTimestamps != true {
		t.Errorf("expected merged ApplyFileTimestamps to be true, got %v", merged.Options.ApplyFileTimestamps)
	}
}

func TestSetProfileAndSetConfigPath(t *testing.T) {
	// Reset after test
	defer func() {
		SetProfile("")
		SetConfigPath("")
	}()

	SetProfile("vip_account")
	path := GetConfigPath()
	dir := GetConfigDir()
	if filepath.Base(path) != "config.toml" {
		t.Errorf("expected config.toml, got %s", filepath.Base(path))
	}
	if filepath.Base(dir) != "vip_account" {
		t.Errorf("expected directory vip_account, got %s", filepath.Base(dir))
	}

	custom := filepath.Join(t.TempDir(), "custom_conf.toml")
	SetConfigPath(custom)
	if GetConfigPath() != custom {
		t.Errorf("expected %s, got %s", custom, GetConfigPath())
	}
	if GetConfigDir() != filepath.Dir(custom) {
		t.Errorf("expected %s, got %s", filepath.Dir(custom), GetConfigDir())
	}
}

func TestEnsureConfigUpdatedKeepsExplicitFalse(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	minimal := `[account]
auth_token = "dummy_token"
user_agent = "dummy_agent"

[options]
save_location = "/tmp/downloads"

[live_settings]
record_chat = false
ffmpeg_convert = false
`
	if err := os.WriteFile(configPath, []byte(minimal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfigUpdated(configPath); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LiveSettings.RecordChat || cfg.LiveSettings.FFmpegConvert || !cfg.LiveSettings.GenerateContactSheet || cfg.LiveSettings.VODsFileExtension != ".ts" {
		t.Fatalf("live settings after upgrade: %+v", cfg.LiveSettings)
	}
}

func TestSaveConfigKeepsFilePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix permission bits")
	}
	configPath := filepath.Join(t.TempDir(), "config.toml")
	SetConfigPath(configPath)
	defer SetConfigPath("")
	cfg := CreateDefaultConfig()
	cfg.Account.AuthToken, cfg.Account.UserAgent = "token", "agent"
	for range 2 {
		if err := SaveConfig(cfg); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("config holding the auth token has mode %04o", perm)
	}
}
