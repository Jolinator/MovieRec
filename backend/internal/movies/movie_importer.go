package movies

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"backend/internal/common"
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

func ImportPopularMoviesWithRetry(ctx context.Context, moviesDB *MovieRepository, attempts int, delay time.Duration) error {
	if attempts < 1 {
		return errors.New("movie import attempts must be at least 1")
	}
	if delay < 0 {
		return errors.New("movie import retry delay cannot be negative")
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := ImportPopularMovies(ctx, moviesDB)
		if err == nil {
			return nil
		}
		lastErr = err

		if !errors.Is(err, common.ErrMoviesDatabaseUnavailable) || attempt == attempts {
			break
		}

		log.Printf("movie import attempt %d/%d failed because the movies database is unavailable; retrying in %s", attempt, attempts, delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("movie import failed after %d attempts: %w", attempts, lastErr)
}
