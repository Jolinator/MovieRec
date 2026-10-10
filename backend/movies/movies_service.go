package movies

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
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

func parseMoviePagination(query url.Values) (MoviePagination, error) {
	pagination := MoviePagination{
		Limit:  defaultMovieLimit,
		Offset: 0,
	}

	for name, target := range map[string]*int{
		"limit":  &pagination.Limit,
		"offset": &pagination.Offset,
	} {
		value := query.Get(name)
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return MoviePagination{}, fmt.Errorf(
				"%w: %s must be an integer",
				common.ErrInvalidMoviePagination,
				name,
			)
		}
		*target = parsed
	}
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
