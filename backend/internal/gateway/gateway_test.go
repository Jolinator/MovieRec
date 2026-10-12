package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"backend/internal/common"
)

func upstream(t *testing.T, handler http.HandlerFunc) *url.URL {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	parsed, _ := url.Parse(server.URL)
	return parsed
}

func nameHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", name)
		w.Header().Add("Set-Cookie", "access_token=abc; Path=/")
		_, _ = w.Write([]byte(r.Method + " " + r.URL.Path + " cookie=" + r.Header.Get("Cookie")))
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Auth:            Upstream{Public: upstream(t, nameHandler("auth")), Internal: upstream(t, nameHandler("auth-internal"))},
		Movies:          Upstream{Public: upstream(t, nameHandler("movies")), Internal: upstream(t, nameHandler("movies-internal"))},
		Recommendations: Upstream{Public: upstream(t, nameHandler("recommendations")), Internal: upstream(t, nameHandler("recommendations-internal"))},
	}
}

func deadURL() *url.URL {
	server := httptest.NewServer(nil)
	parsed, _ := url.Parse(server.URL)
	server.Close()
	return parsed
}

func TestRoutesReachTheOwningService(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, testConfig(t))

	tests := []struct {
		method, path, want string
	}{
		{"POST", "/movieAPI/login", "auth"},
		{"POST", "/movieAPI/signup", "auth"},
		{"POST", "/movieAPI/logout", "auth"},
		{"GET", "/movieAPI/session", "auth"},
		{"GET", "/movieAPI/users", "auth"},
		{"GET", "/movieAPI/movies", "movies"},
		{"GET", "/movieAPI/genres", "movies"},
		{"GET", "/movieAPI/users/abc/list", "recommendations"},
		{"GET", "/movieAPI/my-list", "recommendations"},
		{"POST", "/movieAPI/my-list", "recommendations"},
		{"DELETE", "/movieAPI/my-list/42", "recommendations"},
		{"POST", "/movieAPI/recommendations/chat", "recommendations"},
		{"GET", "/movieAPI/recommendations", "recommendations"},
		{"GET", "/movieAPI/recommendations/ready", "recommendations"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Cookie", "access_token=jwt")
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("X-Upstream"); got != test.want {
				t.Fatalf("routed to %q, want %q (status %d)", got, test.want, recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), "cookie=access_token=jwt") {
				t.Fatalf("cookie not forwarded: %q", recorder.Body.String())
			}
			if recorder.Header().Get("Set-Cookie") == "" {
				t.Fatal("Set-Cookie not passed back")
			}
		})
	}
}

func TestInternalAPIIsNotExposed(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, testConfig(t))

	for _, path := range []string{
		"/internal/movies",
		"/internal/health",
		"/internal/recommendations/users/6f1e8c1a-0000-0000-0000-000000000000/ensure-fresh",
		"/movieAPI/internal/movies",
	} {
		for _, method := range []string{"GET", "POST"} {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
			if recorder.Header().Get("X-Upstream") != "" {
				t.Fatalf("%s %s was proxied to %q", method, path, recorder.Header().Get("X-Upstream"))
			}
		}
	}
}

func TestUnreachableUpstreamIsServiceUnavailable(t *testing.T) {
	config := testConfig(t)
	config.Movies.Public = deadURL()

	mux := http.NewServeMux()
	RegisterRoutes(mux, config)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/movieAPI/movies", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

func healthUpstream(t *testing.T, health common.ServiceHealth, queries *[]string) *url.URL {
	return upstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != common.InternalHealthPath {
			http.NotFound(w, r)
			return
		}
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(health)
	})
}

func getHealth(t *testing.T, mux *http.ServeMux, target string) (int, common.HealthResponse) {
	t.Helper()
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("GET", target, nil))
	var health common.HealthResponse
	if recorder.Code == http.StatusOK {
		if err := json.NewDecoder(recorder.Body).Decode(&health); err != nil {
			t.Fatal(err)
		}
	}
	return recorder.Code, health
}

func TestHealthMergesServices(t *testing.T) {
	config := testConfig(t)
	config.Auth.Internal = healthUpstream(t, common.ServiceHealth{DatabaseAvailable: true, Ready: true}, nil)
	config.Movies.Internal = healthUpstream(t, common.ServiceHealth{DatabaseAvailable: true, Ready: true}, nil)
	config.Recommendations.Internal = healthUpstream(t, common.ServiceHealth{DatabaseAvailable: false, Ready: false}, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, config)
	_, got := getHealth(t, mux, "/movieAPI/health")

	want := common.HealthResponse{
		Message: "API is healthy!", BackendAvailable: true, ImportedMoviesAvailable: true,
		AuthDBAvailable: true, MoviesDBAvailable: true,
	}
	if got != want {
		t.Fatalf("health = %+v, want %+v", got, want)
	}
}

func TestHealthReportsDownServiceAsUnavailable(t *testing.T) {
	config := testConfig(t)
	config.Auth.Internal = healthUpstream(t, common.ServiceHealth{DatabaseAvailable: true, Ready: true}, nil)
	config.Movies.Internal = healthUpstream(t, common.ServiceHealth{DatabaseAvailable: true, Ready: true}, nil)
	config.Recommendations.Internal = deadURL()

	mux := http.NewServeMux()
	RegisterRoutes(mux, config)
	code, got := getHealth(t, mux, "/movieAPI/health")

	if code != http.StatusOK || !got.BackendAvailable || !got.AuthDBAvailable || got.RecommendationsDBAvailable || got.RecommendationsReady {
		t.Fatalf("status %d, health %+v", code, got)
	}
}

func TestHealthScanIsForwardedPerDatabase(t *testing.T) {
	var authQueries, moviesQueries, recommendationsQueries []string
	config := testConfig(t)
	config.Auth.Internal = healthUpstream(t, common.ServiceHealth{}, &authQueries)
	config.Movies.Internal = healthUpstream(t, common.ServiceHealth{}, &moviesQueries)
	config.Recommendations.Internal = healthUpstream(t, common.ServiceHealth{}, &recommendationsQueries)

	mux := http.NewServeMux()
	RegisterRoutes(mux, config)
	getHealth(t, mux, "/movieAPI/health?scan=auth")
	getHealth(t, mux, "/movieAPI/health")

	if strings.Join(authQueries, ",") != "scan=true," || strings.Join(moviesQueries, ",") != "scan=false," || strings.Join(recommendationsQueries, ",") != "scan=false," {
		t.Fatalf("queries auth=%q movies=%q recommendations=%q", authQueries, moviesQueries, recommendationsQueries)
	}

	if code, _ := getHealth(t, mux, "/movieAPI/health?scan=nope"); code != http.StatusBadRequest {
		t.Fatalf("invalid scan status = %d, want 400", code)
	}
}
