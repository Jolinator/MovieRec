package movies

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"backend/common"
)

const (
	defaultMovieLimit = 20
	maxMovieLimit     = 100
)

type MovieService struct {
	repository *MovieRepository
}

type MoviePagination struct {
	Limit   int
	Offset  int
	GenreID *int
}

func NewMovieService(repository *MovieRepository) *MovieService {
	return &MovieService{repository: repository}
}

func (s *MovieService) ListFromQuery(ctx context.Context, query url.Values) ([]common.MovieResponse, error) {
	pagination, err := parseMoviePagination(query)
	if err != nil {
		return nil, err
	}
	return s.List(ctx, pagination)
}

func queryInt(query url.Values, name string, fallback int) (int, error) {
	value := query.Get(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", common.ErrInvalidMoviePagination, name)
	}
	return parsed, nil
}

func parseMoviePagination(query url.Values) (MoviePagination, error) {
	limit, err := queryInt(query, "limit", defaultMovieLimit)
	if err != nil {
		return MoviePagination{}, err
	}
	offset, err := queryInt(query, "offset", 0)
	if err != nil {
		return MoviePagination{}, err
	}
	pagination := MoviePagination{Limit: limit, Offset: offset}
	if value := query.Get("genre"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return MoviePagination{}, fmt.Errorf("%w: genre must be a positive integer", common.ErrInvalidMoviePagination)
		}
		pagination.GenreID = &parsed
	}

	if err := validateMoviePagination(pagination.Limit, pagination.Offset); err != nil {
		return MoviePagination{}, err
	}
	return pagination, nil
}

func (s *MovieService) List(ctx context.Context, pagination MoviePagination) ([]common.MovieResponse, error) {
	if err := validateMoviePagination(pagination.Limit, pagination.Offset); err != nil {
		return nil, err
	}

	movies, err := s.repository.List(ctx, pagination.Limit, pagination.Offset, pagination.GenreID)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}

	response := make([]common.MovieResponse, 0, len(movies))
	for _, movie := range movies {
		response = append(response, MovieResponse(movie))
	}
	return response, nil
}

func (s *MovieService) ListGenres(ctx context.Context) ([]Genre, error) {
	return s.repository.ListGenres(ctx)
}

func (s *MovieService) ListByTMDBIDs(ctx context.Context, tmdbIDs []int) ([]common.MovieResponse, error) {
	movies, err := s.repository.ListByTMDBIDs(ctx, tmdbIDs)
	if err != nil {
		return nil, err
	}
	return MovieResponses(movies), nil
}

// ListByTMDBIDsFromQuery looks up the movies named by the comma-separated "ids" parameter.
func (s *MovieService) ListByTMDBIDsFromQuery(ctx context.Context, query url.Values) ([]common.MovieResponse, error) {
	tmdbIDs, err := parseTMDBIDs(query.Get("ids"))
	if err != nil {
		return nil, err
	}
	return s.ListByTMDBIDs(ctx, tmdbIDs)
}

func parseTMDBIDs(value string) ([]int, error) {
	if value == "" {
		return nil, fmt.Errorf("%w: ids is required", common.ErrInvalidMoviePagination)
	}
	parts := strings.Split(value, ",")
	if len(parts) > maxMovieLimit {
		return nil, fmt.Errorf("%w: ids accepts at most %d values", common.ErrInvalidMoviePagination, maxMovieLimit)
	}
	tmdbIDs := make([]int, len(parts))
	for i, part := range parts {
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%w: ids must be positive integers", common.ErrInvalidMoviePagination)
		}
		tmdbIDs[i] = id
	}
	return tmdbIDs, nil
}

func (s *MovieService) ListSynopses(ctx context.Context, afterTMDBID, limit int) ([]common.MovieSynopsis, error) {
	return s.repository.ListSynopses(ctx, afterTMDBID, limit)
}

// ListSynopsesFromQuery pages through movie synopses using the "after" (TMDB id cursor) and "limit" parameters.
func (s *MovieService) ListSynopsesFromQuery(ctx context.Context, query url.Values) ([]common.MovieSynopsis, error) {
	limit, err := queryInt(query, "limit", maxMovieLimit)
	if err != nil {
		return nil, err
	}
	after, err := queryInt(query, "after", 0)
	if err != nil {
		return nil, err
	}
	if after < 0 {
		return nil, fmt.Errorf("%w: after must be a non-negative integer", common.ErrInvalidMoviePagination)
	}
	if err := validateMoviePagination(limit, 0); err != nil {
		return nil, err
	}
	return s.ListSynopses(ctx, after, limit)
}

func validateMoviePagination(limit, offset int) error {
	if limit < 1 || limit > maxMovieLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d", common.ErrInvalidMoviePagination, maxMovieLimit)
	}
	if offset < 0 {
		return fmt.Errorf("%w: offset must be a non-negative integer", common.ErrInvalidMoviePagination)
	}
	return nil
}

func MovieResponse(movie Movie) common.MovieResponse {
	var releaseDate *string
	if movie.ReleaseDate != nil {
		formattedReleaseDate := movie.ReleaseDate.Format(time.DateOnly)
		releaseDate = &formattedReleaseDate
	}

	return common.MovieResponse{
		TMDBID:        movie.TMDBID,
		Title:         movie.Title,
		ReleaseDate:   releaseDate,
		AverageRating: movie.AverageRating,
		PosterPath:    movie.PosterPath,
		Synopsis:      movie.Synopsis,
		BackdropPath:  movie.BackdropPath,
	}
}

func MovieResponses(input []Movie) []common.MovieResponse {
	response := make([]common.MovieResponse, 0, len(input))
	for _, movie := range input {
		response = append(response, MovieResponse(movie))
	}
	return response
}
