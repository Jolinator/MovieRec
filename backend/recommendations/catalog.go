package recommendations

import (
	"context"
	"fmt"

	"backend/movies"
)

// MovieCatalog is the read access the recommendation service needs to the movies service.
type MovieCatalog interface {
	FindByTMDBID(ctx context.Context, tmdbID int) (movies.Movie, error)
	ListSynopses(ctx context.Context, afterTMDBID, limit int) ([]movies.MovieSynopsis, error)
}

func findMovies(ctx context.Context, catalog MovieCatalog, tmdbIDs []int) ([]movies.Movie, error) {
	found := make([]movies.Movie, 0, len(tmdbIDs))
	for _, id := range tmdbIDs {
		movie, err := catalog.FindByTMDBID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load movie %d: %w", id, err)
		}
		found = append(found, movie)
	}
	return found, nil
}
