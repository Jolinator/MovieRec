package gateway

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

var proxyTransport = &http.Transport{
	DialContext:         (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	MaxIdleConnsPerHost: 32,
	IdleConnTimeout:     60 * time.Second,
	// Kept below the server's write timeout so a slow upstream yields a 504 instead of a dropped connection.
	ResponseHeaderTimeout: 100 * time.Second,
}

// newProxy forwards requests untouched (including cookies) to target and reports unreachable upstreams as 503/504.
func newProxy(target *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.SetXForwarded()
		},
		Transport: proxyTransport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("gateway: %s %s: %v", r.Method, r.URL.Path, err)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				http.Error(w, "service timed out", http.StatusGatewayTimeout)
				return
			}
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		},
	}
}
