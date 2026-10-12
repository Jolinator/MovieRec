package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func corsHandler(t *testing.T, called *bool) http.Handler {
	t.Helper()
	t.Setenv("FRONTEND_PORT", "")
	return CorsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusTeapot)
	}))
}

func TestCorsAllowsFrontendOrigins(t *testing.T) {
	for _, origin := range []string{"http://localhost:5173", "https://localhost:5173"} {
		var called bool
		req := httptest.NewRequest("GET", "/movieAPI/movies", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		corsHandler(t, &called).ServeHTTP(rec, req)

		h := rec.Header()
		if h.Get("Access-Control-Allow-Origin") != origin || h.Get("Vary") != "Origin" ||
			h.Get("Access-Control-Allow-Credentials") != "true" ||
			h.Get("Access-Control-Allow-Methods") != "GET, POST, DELETE, OPTIONS" ||
			h.Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
			t.Errorf("%s: headers = %v", origin, h)
		}
		if !called || rec.Code != http.StatusTeapot {
			t.Errorf("%s: next handler must run (called=%v status=%d)", origin, called, rec.Code)
		}
	}
}

func TestCorsRejectsOtherOrigins(t *testing.T) {
	for _, origin := range []string{"", "http://evil.example", "http://localhost:9999", "http://localhost:5173.evil.com", "null"} {
		var called bool
		req := httptest.NewRequest("GET", "/x", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		corsHandler(t, &called).ServeHTTP(rec, req)
		for _, name := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Allow-Methods"} {
			if rec.Header().Get(name) != "" {
				t.Errorf("origin %q: unexpected %s", origin, name)
			}
		}
		if !called {
			t.Errorf("origin %q: next handler must still run", origin)
		}
	}
}

func TestCorsPreflightShortCircuits(t *testing.T) {
	var called bool
	req := httptest.NewRequest(http.MethodOptions, "/movieAPI/my-list", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	corsHandler(t, &called).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || called {
		t.Fatalf("status = %d, called = %v", rec.Code, called)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("preflight must carry CORS headers")
	}
}

func TestCorsUsesConfiguredFrontendPort(t *testing.T) {
	t.Setenv("FRONTEND_PORT", "3000")
	handler := CorsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for origin, want := range map[string]string{
		"http://localhost:3000": "http://localhost:3000",
		"http://localhost:5173": "",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Errorf("origin %s: got %q, want %q", origin, got, want)
		}
	}
}

func TestRedirectToMovieAPI(t *testing.T) {
	tests := []struct {
		path         string
		wantStatus   int
		wantLocation string
	}{
		{"/", http.StatusTemporaryRedirect, "/movieAPI/"},
		{"/movies", http.StatusTemporaryRedirect, "/movieAPI/movies"},
		{"/a/b", http.StatusTemporaryRedirect, "/movieAPI/a/b"},
		{"/movieAPI/unknown", http.StatusNotFound, ""},
		{"/movieAPIextra", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		RedirectToMovieAPI(rec, httptest.NewRequest("GET", tt.path, nil))
		if rec.Code != tt.wantStatus || rec.Header().Get("Location") != tt.wantLocation {
			t.Errorf("%s: status = %d, location = %q", tt.path, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestMetricsAndRedirectRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, testConfig(t))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/metrics = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/nothing-here", nil))
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("unknown path = %d, want 307", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/movieAPI/login", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("wrong method = %d, want 404", rec.Code)
	}
}
