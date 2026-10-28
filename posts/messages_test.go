package posts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/utils"
	"golang.org/x/time/rate"
)

type testRoundTripper struct{ body string }

func (f testRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(f.body)), Header: http.Header{}, Request: r}, nil
}

func TestDeterministicMessageMediaOrder(t *testing.T) {
	ids := []string{"m1", "m2", "m3", "m4", "m5", "m6"}
	var media []string
	for _, id := range ids {
		media = append(media, fmt.Sprintf(`{"id":"%s","media":{"id":"x%s","mimetype":"image/jpeg","locations":[{"location":"https://cdn.example.com/%s.jpg"}]}}`, id, id, id))
	}
	body := fmt.Sprintf(`{"success":true,"response":{"messages":[{"id":"msg1","attachments":[{"contentType":2,"contentId":"b1"}]}],"accountMediaBundles":[{"id":"b1","accountMediaIds":["m1","m2","m3","m4","m5","m6"]}],"accountMedia":[%s]}}`, strings.Join(media, ","))

	old := utils.HTTPClient.Transport
	utils.HTTPClient.Transport = testRoundTripper{body}
	defer func() { utils.HTTPClient.Transport = old }()

	oldLimiter := messageLimiter
	messageLimiter = rate.NewLimiter(rate.Inf, 1)
	defer func() { messageLimiter = oldLimiter }()

	for run := 0; run < 5; run++ {
		batch, _, err := getMessageBatchWithMedia(context.Background(), "g1", "0", &headers.FanslyHeaders{})
		if err != nil {
			t.Fatalf("getMessageBatchWithMedia failed: %v", err)
		}
		if len(batch) != 1 || len(batch[0].Media) != 6 {
			t.Fatalf("unexpected batch result: %+v", batch)
		}
		for i, m := range batch[0].Media {
			if m.ID != ids[i] {
				t.Errorf("run %d index %d: expected %s, got %s", run, i, ids[i], m.ID)
			}
		}
	}
}
