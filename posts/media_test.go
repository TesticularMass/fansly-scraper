package posts

import (
	"encoding/json"
	"testing"
)

func TestMediaLocationFilters(t *testing.T) {
	for _, tt := range []struct {
		name, body     string
		story, message bool
	}{
		{"empty", `{}`, false, false},
		{"main", `{"media":{"locations":[{"location":"image.jpg"}]}}`, true, true},
		{"variant", `{"media":{"variants":[{}, {"locations":[{"location":"video.mp4"}]}]}}`, true, true},
		{"preview", `{"preview":{"locations":[{"location":"preview.jpg"}]}}`, true, true},
		{"preview variant", `{"preview":{"variants":[{"locations":[{"location":"preview.jpg"}]}]}}`, true, true},
		{"empty preview", `{"preview":{}}`, false, false},
		{"HLS without URL", `{"media":{"type":2,"variants":[{"type":302}]}}`, false, true},
		{"DASH without URL", `{"media":{"type":2,"variants":[{"type":303}]}}`, false, true},
		{"nonvideo manifest", `{"media":{"type":1,"variants":[{"type":302}]}}`, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var media AccountMedia
			if err := json.Unmarshal([]byte(tt.body), &media); err != nil {
				t.Fatal(err)
			}
			if got := len(filterValidAccountMedia([]AccountMedia{media})) > 0; got != tt.story {
				t.Errorf("story retained = %v, want %v", got, tt.story)
			}
			if got := len(filterMediaWithLocations([]AccountMedia{media})) > 0; got != tt.message {
				t.Errorf("message retained = %v, want %v", got, tt.message)
			}
		})
	}
}
