package database

import (
	"context"
	"errors"
	"testing"
)

func newStatuses(auth, movies *fakePinger) Statuses {
	return Statuses{
		AuthDatabase:   NewDBStatus(auth),
		MoviesDatabase: NewDBStatus(movies),
	}
}

func TestStatusesScanOnlyTargets(t *testing.T) {
	auth, movies := &fakePinger{}, &fakePinger{}
	statuses := newStatuses(auth, movies)

	statuses.Scan(context.Background(), AuthDatabase)
	if auth.calls.Load() != 1 || movies.calls.Load() != 0 {
		t.Fatalf("auth=%d movies=%d", auth.calls.Load(), movies.calls.Load())
	}
	if !statuses[AuthDatabase].Available() || statuses[MoviesDatabase].Available() {
		t.Fatal("only the scanned database should be available")
	}
}

func TestStatusesScanIgnoresUnknownTargets(t *testing.T) {
	statuses := newStatuses(&fakePinger{}, &fakePinger{})
	statuses.Scan(context.Background(), RecommendationsDatabase) // must not panic
}

func TestStatusesRefreshRescansOnlyUnavailable(t *testing.T) {
	auth, movies := &fakePinger{}, &fakePinger{}
	statuses := newStatuses(auth, movies)
	statuses.Scan(context.Background(), AuthDatabase)
	movies.set(errors.New("down"))

	statuses.Refresh(context.Background())
	if auth.calls.Load() != 1 {
		t.Errorf("available db was rescanned (%d)", auth.calls.Load())
	}
	if movies.calls.Load() != 1 {
		t.Errorf("unavailable db was not rescanned (%d)", movies.calls.Load())
	}
}
