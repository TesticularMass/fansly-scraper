package utils

import (
	"net"
	"net/http"
	"time"
)

// HTTPClient is the shared client for all outbound requests. It bounds
// dial/TLS/response-header waits so a dead connection can't hang a goroutine
// forever, but sets no overall timeout because large media downloads can
// legitimately take longer than any fixed limit.
var HTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   10,
	},
}

// ParseUnixTimestamp parses both Unix seconds and Unix milliseconds timestamps.
func ParseUnixTimestamp(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	if ts > 1e11 {
		return time.Unix(ts/1000, (ts%1000)*1000000)
	}
	return time.Unix(ts, 0)
}
