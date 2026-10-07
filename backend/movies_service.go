package main

import (
	"context"

	"fmt"
	"net/url"
	"strconv"
)


const (
	defaultMovieLimit = 20
	maxMovieLimit     = 100
)

type MovieService struct {
	repository *MovieRepository
}

type MoviePagination struct {
	Limit  int
	Offset int
}

func NewMovieService(repository *MovieRepository) *MovieService {
	return &MovieService{repository: repository}
}

func (s *MovieService) ListFromQuery(ctx context.Context, query url.Values) ([]MovieResponse, error) {
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
				ErrInvalidMoviePagination,
				name,
			)
		}
		*target = parsed
	}

	if err := validateMoviePagination(pagination.Limit, pagination.Offset); err != nil {
		return MoviePagination{}, err
	}
	return pagination, nil
}

func (s *MovieService) List(ctx context.Context, pagination MoviePagination) ([]MovieResponse, error) {
	if err := validateMoviePagination(pagination.Limit, pagination.Offset); err != nil {
		return nil, err
	}

	movies, err := s.repository.List(ctx, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}

	response := make([]MovieResponse, 0, len(movies))
	for _, movie := range movies {
		response = append(response, movieResponse(movie))
	}
	return response, nil
}

func validateMoviePagination(limit, offset int) error {
	if limit < 1 || limit > maxMovieLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidMoviePagination, maxMovieLimit)
	}
	if offset < 0 {
		return fmt.Errorf("%w: offset must be a non-negative integer", ErrInvalidMoviePagination)
	}
	return nil
}

func movieResponse(movie Movie) MovieResponse {
	return MovieResponse{
		TMDBID:        movie.TMDBID,
		Title:         movie.Title,
		AverageRating: movie.AverageRating,
		PosterPath:    movie.PosterPath,
		Synopsis:      movie.Synopsis,
		BackdropPath:  movie.BackdropPath,
	}
}
