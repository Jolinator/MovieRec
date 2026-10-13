package recommendations

import (
	"context"
	"fmt"
	"time"

	"backend/internal/common"
	"backend/internal/database"
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

func (r *RecommendationCacheRepository) Request(ctx context.Context, userID uuid.UUID) (err error) {
	defer trackRecommendationDatabaseOperation("request_refresh", time.Now(), &err)
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO user_recommendations (user_id, requested_at, status)
		VALUES ($1, CURRENT_TIMESTAMP, 'pending')
		ON CONFLICT (user_id) DO UPDATE
		SET requested_at = CURRENT_TIMESTAMP, status = 'pending'`, userID)
	return err
}

func (r *RecommendationCacheRepository) Get(ctx context.Context, userID uuid.UUID) (cache recommendationCache, err error) {
	defer trackRecommendationDatabaseOperation("get_recommendation_cache", time.Now(), &err)
	if !r.dbStatus.Available() {
		return recommendationCache{}, common.ErrRecommendationsDatabaseUnavailable
	}
	err = r.db.QueryRow(ctx, `
		SELECT requested_at, generated_at, status
		FROM user_recommendations WHERE user_id = $1`, userID).
		Scan(&cache.RequestedAt, &cache.GeneratedAt, &cache.Status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return recommendationCache{Status: "missing"}, nil
		}
		return recommendationCache{}, err
	}
	cache.MovieIDs, err = r.loadMovieIDs(ctx, userID)
	return cache, err
}

func (r *RecommendationCacheRepository) loadMovieIDs(ctx context.Context, userID uuid.UUID) ([]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tmdb_id FROM user_recommendation_items
		WHERE user_id = $1 ORDER BY position`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var movieIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		movieIDs = append(movieIDs, id)
	}
	return movieIDs, rows.Err()
}

func (r *RecommendationCacheRepository) Save(ctx context.Context, userID uuid.UUID, status string, movieIDs []int) (err error) {
	defer trackRecommendationDatabaseOperation("save_recommendation_cache", time.Now(), &err)
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := updateRecommendationStatus(ctx, tx, userID, status); err != nil {
		return err
	}
	if err := deleteRecommendationItems(ctx, tx, userID); err != nil {
		return err
	}
	if err := insertRecommendationItems(ctx, tx, userID, movieIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func updateRecommendationStatus(ctx context.Context, tx pgx.Tx, userID uuid.UUID, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE user_recommendations
		SET generated_at = CURRENT_TIMESTAMP, status = $2
		WHERE user_id = $1`, userID, status)
	return err
}

func deleteRecommendationItems(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM user_recommendation_items WHERE user_id = $1`, userID)
	return err
}

func insertRecommendationItems(ctx context.Context, tx pgx.Tx, userID uuid.UUID, movieIDs []int) error {
	for position, movieID := range movieIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_recommendation_items (user_id, tmdb_id, position)
			VALUES ($1, $2, $3)`, userID, movieID, position+1); err != nil {
			return err
		}
	}
	return nil
}

func (r *RecommendationCacheRepository) LoadMovies(ctx context.Context, userID uuid.UUID, catalog MovieCatalog) ([]common.MovieResponse, error) {
	cache, err := r.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	cached, err := catalog.ListByTMDBIDs(ctx, cache.MovieIDs)
	if err != nil {
		return nil, fmt.Errorf("load cached recommendations: %w", err)
	}
	return cached, nil
}
