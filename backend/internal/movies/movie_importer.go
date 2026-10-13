package movies

import (
	"context"
	"fmt"
	"log"
	"time"
)

func ImportPopularMovies(ctx context.Context, moviesDB *MovieRepository) error {
	importLimit, err := ImportLimit()
	if err != nil {
		return fmt.Errorf("read TMDB import limit: %w", err)
	}

	tmdbClient, err := NewTMDBClient(nil)
	if err != nil {
		return fmt.Errorf("create TMDB client: %w", err)
	}

	tmdbMovies, err := tmdbClient.DiscoverPopularMovies(ctx, importLimit)
	if err != nil {
		return fmt.Errorf("fetch popular TMDB movies: %w", err)
	}
	if len(tmdbMovies) == 0 {
		return fmt.Errorf("TMDB returned no popular movies")
	}

	for _, tmdbMovie := range tmdbMovies {
		movie, err := mapTMDBMovie(tmdbMovie)
		if err != nil {
			return fmt.Errorf("map TMDB movie %d: %w", tmdbMovie.ID, err)
		}
		if err := moviesDB.Upsert(ctx, movie); err != nil {
			return fmt.Errorf("persist TMDB movie %d: %w", movie.TMDBID, err)
		}
	}

	log.Printf("imported %d popular movies from TMDB", len(tmdbMovies))
	return nil
}

// ImportPopularMoviesUntilSuccess retries the initial catalogue import until it
// succeeds or the service is shutting down. The capped backoff lets transient
// database and TMDB failures recover without keeping the service unready forever.
func ImportPopularMoviesUntilSuccess(ctx context.Context, moviesDB *MovieRepository) error {
	const initialDelay = 2 * time.Second
	const maxDelay = 30 * time.Second

	delay := initialDelay
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ImportPopularMovies(ctx, moviesDB); err == nil {
			return nil
		} else {
			log.Printf("movie import attempt %d failed: %v; retrying in %s", attempt, err, delay)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}
