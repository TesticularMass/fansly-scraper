package posts

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/utils"
	"golang.org/x/time/rate"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOrderedMediaIDsUsesBundleIDsWithoutPositions(t *testing.T) {
	attachments := []Attachment{{ContentType: 2, ContentID: "bundle"}, {ContentType: 1, ContentID: "m1"}}
	bundles := map[string]AccountMediaBundle{"bundle": {ID: "bundle", AccountMediaIDs: []string{"m1", "m2"}}}

	if got := orderedMediaIDs(attachments, bundles); !reflect.DeepEqual(got, []string{"m1", "m2"}) {
		t.Fatalf("orderedMediaIDs = %v, want [m1 m2]", got)
	}
}

func TestGetFullPostDetailsReportsMissingMedia(t *testing.T) {
	oldTransport, oldLimiter := utils.HTTPClient.Transport, limiter
	defer func() { utils.HTTPClient.Transport, limiter = oldTransport, oldLimiter }()
	limiter = rate.NewLimiter(rate.Inf, 1)
	utils.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := http.StatusOK, `{"success":true,"response":{"posts":[{"id":"post","attachments":[{"contentType":1,"contentId":"m1"},{"contentType":1,"contentId":"m2"}]}],"accountMedia":[{"id":"m1"}]}}`
		if r.URL.Path == "/api/v1/account/media" {
			status, body = http.StatusInternalServerError, `{}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})

	post, media, err := GetFullPostDetails(context.Background(), "post", &headers.FanslyHeaders{})
	if !errors.Is(err, ErrMissingMedia) || post == nil || len(media) != 1 {
		t.Fatalf("got post=%v media=%d err=%v, want partial media and ErrMissingMedia", post != nil, len(media), err)
	}
}
