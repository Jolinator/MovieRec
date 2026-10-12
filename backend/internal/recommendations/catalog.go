package recommendations

import (
	"context"
	"errors"

	"backend/internal/common"
)

var errCatalogNotReady = errors.New("movie catalogue is not ready")

// MovieCatalog is the read access the recommendation service needs to the movies service.
type MovieCatalog interface {
	// Ready reports whether the catalogue has finished importing and can be read in full.
	Ready(ctx context.Context) (bool, error)
	// ListByTMDBIDs returns the movies in the order requested, or common.ErrMovieNotFound if any is missing.
	ListByTMDBIDs(ctx context.Context, tmdbIDs []int) ([]common.MovieResponse, error)
	ListSynopses(ctx context.Context, afterTMDBID, limit int) ([]common.MovieSynopsis, error)
}
