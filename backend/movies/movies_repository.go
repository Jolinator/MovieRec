package movies

import (
	"context"
	"fmt"
	"math"
	"strings"

	"backend/common"
	"backend/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Movie struct {
	TMDBID        int
	Title         string
	AverageRating float64
	PosterPath    *string
	Synopsis      string
	BackdropPath  *string
	GenreIDs      []int
}

func mapTMDBMovie(movie TMDBMovie) (Movie, error) {
	title := strings.TrimSpace(movie.Title)
	if movie.ID <= 0 {
		return Movie{}, fmt.Errorf("TMDB movie has invalid id %d", movie.ID)
	}
	if title == "" {
		return Movie{}, fmt.Errorf("TMDB movie %d has an empty title", movie.ID)
	}

	rating := math.Round(movie.VoteAverage*10) / 10
	if rating < 0 {
		rating = 0
	}
	if rating > 10 {
		rating = 10
	}

	return Movie{
		TMDBID:        movie.ID,
		Title:         title,
		AverageRating: rating,
		PosterPath:    movie.PosterPath,
		Synopsis:      movie.Overview,
		BackdropPath:  movie.BackdropPath,
		GenreIDs:      movie.GenreIDs,
	}, nil
}

type MovieRepository struct {
	db        *pgxpool.Pool
	db_status *database.DBStatus
}

func NewMovieRepository(db *pgxpool.Pool, dbStatus *database.DBStatus) *MovieRepository {
	return &MovieRepository{db: db, db_status: dbStatus}
}

func (r *MovieRepository) Upsert(ctx context.Context, movie Movie) error {
	if !r.db_status.MoviesAvailable() {
		return common.ErrMoviesDatabaseUnavailable
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin movie upsert transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO movies (
			tmdb_id, title, average_rating, poster_path, synopsis, backdrop_path
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			average_rating = EXCLUDED.average_rating,
			poster_path = EXCLUDED.poster_path,
			synopsis = EXCLUDED.synopsis,
			backdrop_path = EXCLUDED.backdrop_path,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`

	var movieID uuid.UUID
	if err := tx.QueryRow(
		ctx,
		query,
		movie.TMDBID,
		movie.Title,
		movie.AverageRating,
		movie.PosterPath,
		movie.Synopsis,
		movie.BackdropPath,
	).Scan(&movieID); err != nil {
		return fmt.Errorf("upsert TMDB movie %d: %w", movie.TMDBID, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM movie_genres WHERE movie_id = $1`, movieID); err != nil {
		return fmt.Errorf("clear genres for TMDB movie %d: %w", movie.TMDBID, err)
	}
	for _, genreID := range movie.GenreIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO movie_genres (movie_id, genre_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, movieID, genreID); err != nil {
			return fmt.Errorf("save genre %d for TMDB movie %d: %w", genreID, movie.TMDBID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit TMDB movie %d: %w", movie.TMDBID, err)
	}
	return nil
}

func (r *MovieRepository) List(ctx context.Context, limit, offset int, genreID *int) ([]Movie, error) {
	if !r.db_status.MoviesAvailable() {
		return []Movie{}, common.ErrMoviesDatabaseUnavailable
	}

	query := `
		SELECT tmdb_id, title, average_rating, poster_path, synopsis, backdrop_path
		FROM movies
	`
	args := []any{limit, offset}
	if genreID != nil {
		query += `
		JOIN movie_genres ON movie_genres.movie_id = movies.id
		WHERE movie_genres.genre_id = $3
		`
		args = append(args, *genreID)
	}
	query += `ORDER BY average_rating DESC, tmdb_id LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	defer rows.Close()

	movies := make([]Movie, 0, limit)
	for rows.Next() {
		var movie Movie
		if err := rows.Scan(
			&movie.TMDBID,
			&movie.Title,
			&movie.AverageRating,
			&movie.PosterPath,
			&movie.Synopsis,
			&movie.BackdropPath,
		); err != nil {
			return nil, fmt.Errorf("scan movie: %w", err)
		}
		movies = append(movies, movie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate movies: %w", err)
	}
	return movies, nil
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (r *MovieRepository) ListGenres(ctx context.Context) ([]Genre, error) {
	if !r.db_status.MoviesAvailable() {
		return []Genre{}, common.ErrMoviesDatabaseUnavailable
	}

	rows, err := r.db.Query(ctx, `SELECT id, name FROM genres ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	defer rows.Close()

	genres := make([]Genre, 0)
	for rows.Next() {
		var genre Genre
		if err := rows.Scan(&genre.ID, &genre.Name); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		genres = append(genres, genre)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate genres: %w", err)
	}
	return genres, nil
}
