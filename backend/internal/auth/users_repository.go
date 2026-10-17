package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/internal/common"
	"backend/internal/database"
	"backend/internal/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        uuid.UUID
	Username  string
	CreatedAt time.Time
}

type UserCredentials struct {
	User
	PasswordHash string
}

type UserRepository struct {
	db       *pgxpool.Pool
	dbStatus *database.DBStatus
}

func NewUserRepository(db *pgxpool.Pool, dbStatus *database.DBStatus) *UserRepository {
	return &UserRepository{db: db, dbStatus: dbStatus}
}

func (r *UserRepository) Create(ctx context.Context, username, passwordHash string) (user User, err error) {
	defer trackDatabaseOperation("auth", "create_user", time.Now(), &err)
	if !r.dbStatus.Available() {
		return User{}, fmt.Errorf("%w: auth database is unavailable", common.ErrAuthDatabaseUnavailable)
	}

	const query = `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, created_at
	`

	err = r.db.QueryRow(ctx, query, username, passwordHash).
		Scan(&user.ID, &user.Username, &user.CreatedAt)
	if err == nil {
		return user, nil
	}

	if isUniqueViolation(err) {
		return User{}, common.ErrUsernameAlreadyExists
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("%w: insert returned no user", common.ErrUserPersistence)
	}

	return User{}, fmt.Errorf("%w: %v", common.ErrUserPersistence, err)
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (credentials UserCredentials, err error) {
	defer trackDatabaseOperation("auth", "find_user", time.Now(), &err)
	if !r.dbStatus.Available() {
		return UserCredentials{}, fmt.Errorf("%w: auth database is unavailable", common.ErrAuthDatabaseUnavailable)
	}

	const query = `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = $1
	`

	err = r.db.QueryRow(ctx, query, username).Scan(
		&credentials.ID,
		&credentials.Username,
		&credentials.PasswordHash,
		&credentials.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserCredentials{}, common.ErrUserNotFound
	}
	if err != nil {
		return UserCredentials{}, fmt.Errorf("%w: find user: %v", common.ErrUserPersistence, err)
	}

	return credentials, nil
}

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (user User, err error) {
	defer trackDatabaseOperation("auth", "find_user_by_id", time.Now(), &err)
	if !r.dbStatus.Available() {
		return User{}, fmt.Errorf("%w: auth database is unavailable", common.ErrAuthDatabaseUnavailable)
	}

	err = r.db.QueryRow(ctx, `
		SELECT id, username, created_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Username, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, common.ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("%w: find user by id: %v", common.ErrUserPersistence, err)
	}
	return user, nil
}

func (r *UserRepository) SearchByUsername(ctx context.Context, query string, limit int) (users []User, err error) {
	defer trackDatabaseOperation("auth", "search_users", time.Now(), &err)
	if !r.dbStatus.Available() {
		return nil, common.ErrAuthDatabaseUnavailable
	}

	literalQuery := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(query)
	rows, err := r.db.Query(ctx, `
		SELECT id, username, created_at
		FROM users
		WHERE username ILIKE $2 ESCAPE '!'
		ORDER BY CASE
			WHEN lower(username) = lower($1) THEN 0
			WHEN username ILIKE $3 ESCAPE '!' THEN 1
			ELSE 2
		END, username
		LIMIT $4
	`, query, "%"+literalQuery+"%", literalQuery+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("%w: search users: %v", common.ErrUserPersistence, err)
	}
	defer rows.Close()

	users = make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("%w: scan searched user: %v", common.ErrUserPersistence, err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: iterate searched users: %v", common.ErrUserPersistence, err)
	}

	return users, nil
}

func trackDatabaseOperation(database, operation string, started time.Time, err *error) {
	result := "success"
	if *err != nil {
		result = "error"
	}
	observability.ObserveDatabaseOperation(database, operation, result, started)
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}
