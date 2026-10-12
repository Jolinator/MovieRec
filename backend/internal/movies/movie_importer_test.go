package movies

import (
	"context"
	"testing"
	"time"
)

func TestImportPopularMoviesWithRetryRejectsInvalidArguments(t *testing.T) {
	ctx := context.Background()
	if err := ImportPopularMoviesWithRetry(ctx, nil, 0, time.Second); err == nil {
		t.Error("attempts < 1 should fail")
	}
	if err := ImportPopularMoviesWithRetry(ctx, nil, 1, -time.Second); err == nil {
		t.Error("negative delay should fail")
	}
}
