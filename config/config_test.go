package config

import "testing"

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

func TestMergeConfigs(t *testing.T) {
	existing := CreateDefaultConfig()
	existing.Account.AuthToken = "old-token"
	existing.Account.UserAgent = "old-ua"
	existing.Options.SaveLocation = "/old/location"
	existing.Notifications.DiscordWebhook = "https://old.webhook"
	existing.LiveSettings.CheckInterval = 120

	incoming := CreateDefaultConfig()
	incoming.Account.AuthToken = "new-token"
	incoming.Account.UserAgent = "" // Empty must not clobber existing
	incoming.Options.SaveLocation = ""
	incoming.Notifications.DiscordWebhook = ""
	incoming.LiveSettings.CheckInterval = 0 // Unset must fall back to existing

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
}
