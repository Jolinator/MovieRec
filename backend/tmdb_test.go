package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

func TestTMDBClientDiscoverPopularMoviesLimitsResults(t *testing.T) {
	t.Setenv("TMDB_READ_ACCESS_TOKEN", "test-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization header = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("sort_by") != "popularity.desc" {
			t.Errorf("sort_by = %q, want popularity.desc", r.URL.Query().Get("sort_by"))
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":` + strconv.Itoa(page) + `,"total_pages":2,"results":[{"id":1,"title":"One"},{"id":2,"title":"Two"}]}`))
	}))
	defer server.Close()

	client, err := NewTMDBClient(server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL = server.URL

	movies, err := client.DiscoverPopularMovies(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 3 {
		t.Fatalf("got %d movies, want 3", len(movies))
	}
}

func TestTMDBImportLimit(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("TMDB_IMPORT_LIMIT", "")
		limit, err := tmdbImportLimit()
		if err != nil || limit != 500 {
			t.Fatalf("got (%d, %v), want (500, nil)", limit, err)
		}
	})

	t.Run("configured", func(t *testing.T) {
		t.Setenv("TMDB_IMPORT_LIMIT", "250")
		limit, err := tmdbImportLimit()
		if err != nil || limit != 250 {
			t.Fatalf("got (%d, %v), want (250, nil)", limit, err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("TMDB_IMPORT_LIMIT", "zero")
		if _, err := tmdbImportLimit(); err == nil {
			t.Fatal("expected invalid limit error")
		}
	})
}

func TestNewTMDBClientRequiresAPIKey(t *testing.T) {
	original, exists := os.LookupEnv("TMDB_READ_ACCESS_TOKEN")
	_ = os.Unsetenv("TMDB_READ_ACCESS_TOKEN")
	defer func() {
		if exists {
			_ = os.Setenv("TMDB_READ_ACCESS_TOKEN", original)
		}
	}()

	if _, err := NewTMDBClient(nil); err == nil {
		t.Fatal("expected missing API key error")
	}
}
