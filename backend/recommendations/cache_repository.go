package recommendations

import (
	"context"
	"fmt"
	"time"

	"backend/common"
	"backend/database"
	"backend/movies"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecommendationCacheRepository struct {
	db       *pgxpool.Pool
	dbStatus *database.DBStatus
}

func NewRecommendationCacheRepository(db *pgxpool.Pool, dbStatus *database.DBStatus) *RecommendationCacheRepository {
	return &RecommendationCacheRepository{db: db, dbStatus: dbStatus}
}

type recommendationCache struct {
	RequestedAt *time.Time
	GeneratedAt *time.Time
	Status      string
	MovieIDs    []int
}

func (r *RecommendationCacheRepository) Request(ctx context.Context, userID uuid.UUID) error {
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_recommendations (user_id, requested_at, status)
		VALUES ($1, CURRENT_TIMESTAMP, 'pending')
		ON CONFLICT (user_id) DO UPDATE
		SET requested_at = CURRENT_TIMESTAMP, status = 'pending'`, userID)
	return err
}

func (r *RecommendationCacheRepository) Get(ctx context.Context, userID uuid.UUID) (recommendationCache, error) {
	if !r.dbStatus.Available() {
		return recommendationCache{}, common.ErrRecommendationsDatabaseUnavailable
	}
	var cache recommendationCache
	err := r.db.QueryRow(ctx, `
		SELECT requested_at, generated_at, status
		FROM user_recommendations WHERE user_id = $1`, userID).
		Scan(&cache.RequestedAt, &cache.GeneratedAt, &cache.Status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return recommendationCache{Status: "missing"}, nil
		}
		return recommendationCache{}, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT tmdb_id FROM user_recommendation_items
		WHERE user_id = $1 ORDER BY position`, userID)
	if err != nil {
		return recommendationCache{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return recommendationCache{}, err
		}
		cache.MovieIDs = append(cache.MovieIDs, id)
	}
	return cache, rows.Err()
}

func (r *RecommendationCacheRepository) Save(ctx context.Context, userID uuid.UUID, status string, movieIDs []int) error {
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE user_recommendations
		SET generated_at = CURRENT_TIMESTAMP, status = $2
		WHERE user_id = $1`, userID, status); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_recommendation_items WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for position, movieID := range movieIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_recommendation_items (user_id, tmdb_id, position)
			VALUES ($1, $2, $3)`, userID, movieID, position+1); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RecommendationCacheRepository) LoadMovies(ctx context.Context, userID uuid.UUID, movieRepo *movies.MovieRepository) ([]common.MovieResponse, error) {
	cache, err := r.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]common.MovieResponse, 0, len(cache.MovieIDs))
	for _, id := range cache.MovieIDs {
		movie, err := movieRepo.FindByTMDBID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load cached recommendation %d: %w", id, err)
		}
		result = append(result, movies.MovieResponse(movie))
	}
	return result, nil
}
