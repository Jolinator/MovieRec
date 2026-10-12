package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyForwardsRequestUntouched(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody, gotCookie, gotFwd string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotQuery, gotBody = r.Method, r.URL.Path, r.URL.RawQuery, string(body)
		gotCookie = r.Header.Get("Cookie")
		gotFwd = r.Header.Get("X-Forwarded-Host")
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)

	req := httptest.NewRequest("POST", "http://gateway.example/movieAPI/login?a=1", strings.NewReader("payload"))
	req.Header.Set("Cookie", "access_token=abc")
	rec := httptest.NewRecorder()
	newProxy(targetURL).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated || rec.Body.String() != "hello" || rec.Header().Get("X-Upstream") != "yes" {
		t.Fatalf("response = %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	if gotMethod != "POST" || gotPath != "/movieAPI/login" || gotQuery != "a=1" || gotBody != "payload" ||
		gotCookie != "access_token=abc" || gotFwd != "gateway.example" {
		t.Fatalf("upstream saw %s %s ?%s body=%q cookie=%q fwd=%q", gotMethod, gotPath, gotQuery, gotBody, gotCookie, gotFwd)
	}
}

func TestProxyPassesThroughUpstreamErrorStatus(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusConflict)
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	rec := httptest.NewRecorder()
	newProxy(targetURL).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestProxyReportsUnreachableUpstreamWithoutLeakingDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	newProxy(deadURL()).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "dial") || strings.Contains(rec.Body.String(), "127.0.0.1") {
		t.Fatalf("body leaks transport detail: %q", rec.Body.String())
	}
}

func TestProxyTransportSettings(t *testing.T) {
	if proxyTransport.DialContext == nil || proxyTransport.ResponseHeaderTimeout <= 0 ||
		proxyTransport.ResponseHeaderTimeout >= 120*time.Second {
		t.Fatalf("unexpected transport: %+v", proxyTransport)
	}
}
