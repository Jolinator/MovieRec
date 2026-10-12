package common

import (
	"encoding/json"
	"testing"
)

func TestMovieResponseJSONFieldNames(t *testing.T) {
	date, poster := "2020-01-02", "/p.jpg"
	raw, err := json.Marshal(MovieResponse{TMDBID: 1, Title: "T", ReleaseDate: &date, AverageRating: 7.5, PosterPath: &poster, Synopsis: "S"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tmdbId", "title", "releaseDate", "averageRating", "posterPath", "synopsis", "backdropPath"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in %s", key, raw)
		}
	}
	if got["backdropPath"] != nil {
		t.Errorf("nil backdrop should serialize as null, got %v", got["backdropPath"])
	}
}

func TestHealthResponseJSONFieldNames(t *testing.T) {
	raw, _ := json.Marshal(HealthResponse{})
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	for _, key := range []string{"message", "backendAvailable", "importedMoviesAvailable", "authDBAvailable", "moviesDBAvailable", "recommendationsDBAvailable", "recommendationsReady"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
}

func TestServiceHealthAndReadyRoundTrip(t *testing.T) {
	var health ServiceHealth
	if err := json.Unmarshal([]byte(`{"databaseAvailable":true,"ready":true}`), &health); err != nil || !health.DatabaseAvailable || !health.Ready {
		t.Fatalf("health = %+v, err = %v", health, err)
	}
	var ready ReadyResponse
	if err := json.Unmarshal([]byte(`{"ready":true}`), &ready); err != nil || !ready.Ready {
		t.Fatalf("ready = %+v, err = %v", ready, err)
	}
}

func TestInternalPathsAreInternal(t *testing.T) {
	for _, path := range []string{InternalHealthPath, InternalRecommendationsUsersPath} {
		if len(path) < 10 || path[:10] != "/internal/" {
			t.Errorf("path %q is not under /internal/", path)
		}
	}
}
