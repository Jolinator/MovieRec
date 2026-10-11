package recommendations

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"backend/auth"
	"backend/common"
	"backend/movies"
	"github.com/google/uuid"
)

func NewListService(repository *ListRepository, movieRepo *movies.MovieRepository) *ListService {
	return &ListService{repository: repository, movieRepo: movieRepo}
}

func (s *ListService) List(ctx context.Context, userID uuid.UUID) ([]common.MovieResponse, error) {
	return s.ListForUser(ctx, userID)
}

func (s *ListService) ListForUser(ctx context.Context, userID uuid.UUID) ([]common.MovieResponse, error) {
	movieIDs, err := s.repository.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := make([]common.MovieResponse, 0, len(movieIDs))
	for _, tmdbID := range movieIDs {
		movie, err := s.movieRepo.FindByTMDBID(ctx, tmdbID)
		if err != nil {
			return nil, fmt.Errorf("load list movie %d: %w", tmdbID, err)
		}
		response = append(response, movies.MovieResponse(movie))
	}
	return response, nil
}

func (s *ListService) Add(ctx context.Context, userID uuid.UUID, tmdbID int) (common.MovieResponse, error) {
	movie, err := s.movieRepo.FindByTMDBID(ctx, tmdbID)
	if err != nil {
		return common.MovieResponse{}, err
	}
	if err := s.repository.Add(ctx, userID, tmdbID); err != nil {
		return common.MovieResponse{}, err
	}
	return movies.MovieResponse(movie), nil
}

func (s *ListService) Remove(ctx context.Context, userID uuid.UUID, tmdbID int) error {
	return s.repository.Remove(ctx, userID, tmdbID)
}

func userIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, common.ErrInvalidToken
	}
	return userID, nil
}

func ParseTMDBID(value string) (int, error) {
	tmdbID, err := strconv.Atoi(value)
	if err != nil || tmdbID <= 0 {
		return 0, errors.New("tmdbId must be a positive integer")
	}
	return tmdbID, nil
}
