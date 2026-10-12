package recommendations

import (
	"context"
	"errors"
	"strconv"

	"backend/common"
	"github.com/google/uuid"
)

func NewListService(repository *ListRepository, catalog MovieCatalog) *ListService {
	return &ListService{repository: repository, catalog: catalog}
}

func (s *ListService) List(ctx context.Context, userID uuid.UUID) ([]common.MovieResponse, error) {
	return s.ListForUser(ctx, userID)
}

func (s *ListService) ListForUser(ctx context.Context, userID uuid.UUID) ([]common.MovieResponse, error) {
	movieIDs, err := s.repository.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.catalog.ListByTMDBIDs(ctx, movieIDs)
}

func (s *ListService) Add(ctx context.Context, userID uuid.UUID, tmdbID int) (common.MovieResponse, error) {
	movies, err := s.catalog.ListByTMDBIDs(ctx, []int{tmdbID})
	if err != nil {
		return common.MovieResponse{}, err
	}
	if err := s.repository.Add(ctx, userID, tmdbID); err != nil {
		return common.MovieResponse{}, err
	}
	return movies[0], nil
}

func (s *ListService) Remove(ctx context.Context, userID uuid.UUID, tmdbID int) error {
	return s.repository.Remove(ctx, userID, tmdbID)
}

func ParseTMDBID(value string) (int, error) {
	tmdbID, err := strconv.Atoi(value)
	if err != nil || tmdbID <= 0 {
		return 0, errors.New("tmdbId must be a positive integer")
	}
	return tmdbID, nil
}
