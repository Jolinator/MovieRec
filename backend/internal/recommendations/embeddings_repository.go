package recommendations

import (
	"context"
	"fmt"
	"strings"

	"backend/internal/common"
	"backend/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmbeddingRepository stores and searches movie synopsis embeddings in the recommendations database.
type EmbeddingRepository struct {
	db       *pgxpool.Pool
	dbStatus *database.DBStatus
}

func NewEmbeddingRepository(db *pgxpool.Pool, dbStatus *database.DBStatus) *EmbeddingRepository {
	return &EmbeddingRepository{db: db, dbStatus: dbStatus}
}

// Existing reports which of the given movies already have an embedding.
func (r *EmbeddingRepository) Existing(ctx context.Context, tmdbIDs []int) (map[int]bool, error) {
	ids, err := r.queryIDs(ctx, `SELECT tmdb_id FROM movie_embeddings WHERE tmdb_id = ANY($1)`, tmdbIDs)
	if err != nil {
		return nil, err
	}
	existing := make(map[int]bool, len(ids))
	for _, id := range ids {
		existing[id] = true
	}
	return existing, nil
}

func (r *EmbeddingRepository) Save(ctx context.Context, tmdbID int, vector []float32) error {
	if !r.dbStatus.Available() {
		return common.ErrRecommendationsDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO movie_embeddings (tmdb_id, embedding)
		VALUES ($1, $2::vector)
		ON CONFLICT (tmdb_id) DO NOTHING`, tmdbID, vectorLiteral(vector))
	if err != nil {
		return fmt.Errorf("save embedding for movie %d: %w", tmdbID, err)
	}
	return nil
}

// Profile returns the mean embedding of the given movies, or nil when none has an embedding.
func (r *EmbeddingRepository) Profile(ctx context.Context, tmdbIDs []int) ([]float32, error) {
	if !r.dbStatus.Available() {
		return nil, common.ErrRecommendationsDatabaseUnavailable
	}
	var literal *string
	err := r.db.QueryRow(ctx, `SELECT avg(embedding)::text FROM movie_embeddings WHERE tmdb_id = ANY($1)`, tmdbIDs).Scan(&literal)
	if err != nil {
		return nil, fmt.Errorf("build embedding profile: %w", err)
	}
	if literal == nil {
		return nil, nil
	}
	return parseVectorLiteral(*literal)
}

func (r *EmbeddingRepository) SearchSimilar(ctx context.Context, vector []float32, limit int) ([]int, error) {
	return r.queryIDs(ctx, `SELECT tmdb_id FROM movie_embeddings ORDER BY embedding <=> $1::vector LIMIT $2`, vectorLiteral(vector), limit)
}

func (r *EmbeddingRepository) SearchSimilarExcluding(ctx context.Context, vector []float32, excluded []int, limit int) ([]int, error) {
	return r.queryIDs(ctx, `
		SELECT tmdb_id
		FROM movie_embeddings
		WHERE NOT (tmdb_id = ANY($2))
		ORDER BY embedding <=> $1::vector
		LIMIT $3`, vectorLiteral(vector), excluded, limit)
}

func (r *EmbeddingRepository) queryIDs(ctx context.Context, query string, args ...any) ([]int, error) {
	if !r.dbStatus.Available() {
		return nil, common.ErrRecommendationsDatabaseUnavailable
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query movie embeddings: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return nil, fmt.Errorf("read movie embeddings: %w", err)
	}
	return ids, nil
}

func vectorLiteral(vector []float32) string {
	values := make([]string, len(vector))
	for i, value := range vector {
		values[i] = fmt.Sprintf("%g", value)
	}
	return "[" + strings.Join(values, ",") + "]"
}

func parseVectorLiteral(literal string) ([]float32, error) {
	literal = strings.Trim(strings.TrimSpace(literal), "[]")
	if literal == "" {
		return []float32{}, nil
	}
	parts := strings.Split(literal, ",")
	vector := make([]float32, len(parts))
	for i, part := range parts {
		var value float32
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%f", &value); err != nil {
			return nil, fmt.Errorf("parse embedding value: %w", err)
		}
		vector[i] = value
	}
	return vector, nil
}
