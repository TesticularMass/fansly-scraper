package download

import "testing"

func TestParseSegments(t *testing.T) {
	base := "https://cdn.example.com/media/stream.m3u8?Policy=abc&Signature=def"

	content := `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:4.0,
segment_0.ts
#EXTINF:4.0,
segment_1.ts
#EXTINF:4.0,
https://other.example.com/absolute_2.ts?token=xyz
#EXT-X-ENDLIST
`

	urls, err := parseSegments(content, base)
	if err != nil {
		t.Fatalf("parseSegments returned error: %v", err)
	}
	if len(urls) != 3 {
		t.Fatalf("got %d segment URLs, want 3: %v", len(urls), urls)
	}

	// Relative segments resolve against the base and inherit its query (auth tokens)
	want0 := "https://cdn.example.com/media/segment_0.ts?Policy=abc&Signature=def"
	if urls[0] != want0 {
		t.Errorf("urls[0] = %q, want %q", urls[0], want0)
	}

	// Absolute segments with their own query keep it
	want2 := "https://other.example.com/absolute_2.ts?token=xyz"
	if urls[2] != want2 {
		t.Errorf("urls[2] = %q, want %q", urls[2], want2)
	}
}

func TestParseSegmentsSkipsCommentsAndBlank(t *testing.T) {
	content := "#EXTM3U\n\n#EXT-X-ENDLIST\n"
	urls, err := parseSegments(content, "https://cdn.example.com/x.m3u8")
	if err != nil {
		t.Fatalf("parseSegments returned error: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("got %d segment URLs, want 0", len(urls))
	}
}
