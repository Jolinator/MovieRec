package main

import (
	"context"
	"fmt"
	"log"
)

func importPopularMovies(ctx context.Context, moviesDB *MovieRepository) error {
	importLimit, err := tmdbImportLimit()
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
