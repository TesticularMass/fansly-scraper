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
