package movies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestImportLimit(t *testing.T) {
	tests := []struct {
		value   string
		want    int
		wantErr bool
	}{
		{"", 500, false}, {"10", 10, false}, {"0", 0, true}, {"-5", 0, true}, {"abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("TMDB_IMPORT_LIMIT", tt.value)
			got, err := ImportLimit()
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got %d, %v", got, err)
			}
		})
	}
}

func TestNewTMDBClient(t *testing.T) {
	t.Setenv("TMDB_READ_ACCESS_TOKEN", "")
	if _, err := NewTMDBClient(nil); err == nil {
		t.Fatal("expected error without token")
	}

	t.Setenv("TMDB_READ_ACCESS_TOKEN", "tok")
	client, err := NewTMDBClient(nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.readToken != "tok" || client.httpClient == nil || client.httpClient.Timeout <= 0 || client.baseURL != tmdbDiscoverURL {
		t.Fatalf("client = %+v", client)
	}

	custom := &http.Client{}
	client, _ = NewTMDBClient(custom)
	if client.httpClient != custom {
		t.Fatal("custom http client not used")
	}
}

// tmdbServer serves pages of perPage movies, totalPages in total, and counts requests.
func tmdbServer(t *testing.T, perPage, totalPages int, requests *atomic.Int32) *TMDBClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		query := r.URL.Query()
		if query.Get("sort_by") != "popularity.desc" || query.Get("language") != "en-US" {
			t.Errorf("query = %v", query)
		}
		page, _ := strconv.Atoi(query.Get("page"))
		results := make([]TMDBMovie, 0, perPage)
		if page <= totalPages {
			for i := 0; i < perPage; i++ {
				results = append(results, TMDBMovie{ID: (page-1)*perPage + i + 1, Title: "m"})
			}
		}
		_ = json.NewEncoder(w).Encode(tmdbDiscoverResponse{Page: page, TotalPages: totalPages, Results: results})
	}))
	t.Cleanup(server.Close)
	return &TMDBClient{readToken: "secret", httpClient: server.Client(), baseURL: server.URL}
}

func TestDiscoverPopularMoviesTrimsToLimit(t *testing.T) {
	var requests atomic.Int32
	client := tmdbServer(t, 20, 50, &requests)
	movies, err := client.DiscoverPopularMovies(context.Background(), 45)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 45 || movies[0].ID != 1 || movies[44].ID != 45 {
		t.Fatalf("got %d movies", len(movies))
	}
	if requests.Load() != 3 {
		t.Fatalf("requests = %d, want 3", requests.Load())
	}
}

func TestDiscoverPopularMoviesStopsAtTotalPages(t *testing.T) {
	var requests atomic.Int32
	client := tmdbServer(t, 5, 2, &requests)
	movies, err := client.DiscoverPopularMovies(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 10 || requests.Load() != 2 {
		t.Fatalf("movies=%d requests=%d", len(movies), requests.Load())
	}
}

func TestDiscoverPopularMoviesStopsOnEmptyPage(t *testing.T) {
	var requests atomic.Int32
	client := tmdbServer(t, 0, 5, &requests)
	movies, err := client.DiscoverPopularMovies(context.Background(), 10)
	if err != nil || len(movies) != 0 || requests.Load() != 1 {
		t.Fatalf("movies=%d requests=%d err=%v", len(movies), requests.Load(), err)
	}
}

func TestDiscoverPopularMoviesRejectsNonPositiveLimit(t *testing.T) {
	client := &TMDBClient{}
	for _, limit := range []int{0, -1} {
		if _, err := client.DiscoverPopularMovies(context.Background(), limit); err == nil {
			t.Errorf("limit %d should fail", limit)
		}
	}
}

func TestDiscoverPopularMoviesErrors(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"http error": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusUnauthorized) },
		"bad json":   func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{")) },
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			client := &TMDBClient{readToken: "x", httpClient: server.Client(), baseURL: server.URL}
			if _, err := client.DiscoverPopularMovies(context.Background(), 5); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDiscoverPopularMoviesTransportAndRequestErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	client := &TMDBClient{readToken: "x", httpClient: http.DefaultClient, baseURL: url}
	if _, err := client.DiscoverPopularMovies(context.Background(), 5); err == nil {
		t.Fatal("expected transport error")
	}

	client = &TMDBClient{readToken: "x", httpClient: http.DefaultClient, baseURL: "http://bad host"}
	if _, err := client.DiscoverPopularMovies(context.Background(), 5); err == nil {
		t.Fatal("expected request creation error")
	}
}

func TestDiscoverPopularMoviesHonorsContext(t *testing.T) {
	var requests atomic.Int32
	client := tmdbServer(t, 5, 5, &requests)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.DiscoverPopularMovies(ctx, 5); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
