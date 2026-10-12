package recommendations

import (
	"context"
	"errors"
	"fmt"

	"backend/common"
	"backend/database"
	"backend/movies"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ListRepository struct {
	db       *pgxpool.Pool
	dbStatus *database.DBStatus
}

func NewListRepository(db *pgxpool.Pool, dbStatus *database.DBStatus) *ListRepository {
	return &ListRepository{db: db, dbStatus: dbStatus}
}

func (r *ListRepository) List(ctx context.Context, userID uuid.UUID) ([]int, error) {
	return r.ListByUserID(ctx, userID)
}

func (r *ListRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]int, error) {
	if !r.dbStatus.Available() {
		return nil, common.ErrRecommendationsDatabaseUnavailable
	}

	rows, err := r.db.Query(ctx, `
		SELECT items.tmdb_id
		FROM movie_lists lists
		JOIN movie_list_items items ON items.list_id = lists.id
		WHERE lists.user_id = $1
		ORDER BY items.added_at, items.tmdb_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: list movies: %v", common.ErrMovieListPersistence, err)
	}
	defer rows.Close()

	movieIDs := make([]int, 0)
	for rows.Next() {
		var tmdbID int
		if err := rows.Scan(&tmdbID); err != nil {
			return nil, fmt.Errorf("%w: scan movie: %v", common.ErrMovieListPersistence, err)
		}
		movieIDs = append(movieIDs, tmdbID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: iterate movies: %v", common.ErrMovieListPersistence, err)
	}
	return movieIDs, nil
}

func (r *ListRepository) Add(ctx context.Context, userID uuid.UUID, tmdbID int) error {
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: begin add transaction: %v", common.ErrMovieListPersistence, err)
	}
	defer tx.Rollback(ctx)

	var listID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO movie_lists (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, userID).Scan(&listID); err != nil {
		return fmt.Errorf("%w: create list: %v", common.ErrMovieListPersistence, err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO movie_list_items (list_id, tmdb_id)
		VALUES ($1, $2)
	`, listID, tmdbID); err != nil {
		if isUniqueViolation(err) {
			return common.ErrMovieAlreadyInList
		}
		return fmt.Errorf("%w: add movie: %v", common.ErrMovieListPersistence, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: commit add transaction: %v", common.ErrMovieListPersistence, err)
	}
	return nil
}

func (r *ListRepository) Remove(ctx context.Context, userID uuid.UUID, tmdbID int) error {
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}

	tag, err := r.db.Exec(ctx, `
		DELETE FROM movie_list_items
		WHERE list_id = (SELECT id FROM movie_lists WHERE user_id = $1)
		AND tmdb_id = $2
	`, userID, tmdbID)
	if err != nil {
		return fmt.Errorf("%w: remove movie: %v", common.ErrMovieListPersistence, err)
	}
	if tag.RowsAffected() == 0 {
		return common.ErrMovieNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}

type ListService struct {
	repository *ListRepository
	movieRepo  *movies.MovieRepository
}
