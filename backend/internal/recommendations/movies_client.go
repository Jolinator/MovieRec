package recommendations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/common"
)

// maxIDsPerRequest matches the limit of the movies service's by-ids endpoint.
const maxIDsPerRequest = 100

// MoviesClient reads the movie catalogue over the movies service's internal HTTP API.
type MoviesClient struct {
	baseURL string
	client  *http.Client
}

func NewMoviesClient() *MoviesClient {
	defaultURL := "http://localhost:" + env("INTERNAL_PORT", "8081")
	return &MoviesClient{
		baseURL: strings.TrimRight(env("MOVIES_INTERNAL_URL", defaultURL), "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *MoviesClient) Ready(ctx context.Context) (bool, error) {
	var response common.ReadyResponse
	if err := c.get(ctx, "/internal/movies/ready", &response); err != nil {
		return false, err
	}
	return response.Ready, nil
}

func (c *MoviesClient) ListSynopses(ctx context.Context, afterTMDBID, limit int) ([]common.MovieSynopsis, error) {
	var synopses []common.MovieSynopsis
	path := fmt.Sprintf("/internal/movies/synopses?after=%d&limit=%d", afterTMDBID, limit)
	if err := c.get(ctx, path, &synopses); err != nil {
		return nil, err
	}
	return synopses, nil
}

func (c *MoviesClient) ListByTMDBIDs(ctx context.Context, tmdbIDs []int) ([]common.MovieResponse, error) {
	movies := make([]common.MovieResponse, 0, len(tmdbIDs))
	for start := 0; start < len(tmdbIDs); start += maxIDsPerRequest {
		var page []common.MovieResponse
		ids := tmdbIDs[start:min(start+maxIDsPerRequest, len(tmdbIDs))]
		if err := c.get(ctx, "/internal/movies?ids="+joinIDs(ids), &page); err != nil {
			return nil, err
		}
		movies = append(movies, page...)
	}
	return movies, nil
}

func (c *MoviesClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return moviesError(doJSON(c.client, req, target))
}

// moviesError maps transport and HTTP failures to the same errors the in-process catalogue returned.
func moviesError(err error) error {
	var status *httpStatusError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &status) && status.code == http.StatusNotFound:
		return common.ErrMovieNotFound
	case errors.As(err, &status) && status.code != http.StatusServiceUnavailable:
		return err
	default:
		return fmt.Errorf("%w: %v", common.ErrMoviesDatabaseUnavailable, err)
	}
}

func joinIDs(ids []int) string {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = strconv.Itoa(id)
	}
	return strings.Join(values, ",")
}
