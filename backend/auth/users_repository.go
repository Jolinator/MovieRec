package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/common"
	"backend/database"
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

func (r *UserRepository) Create(ctx context.Context, username, passwordHash string) (User, error) {
	if !r.dbStatus.AuthAvailable() {
		return User{}, fmt.Errorf("%w: auth database is unavailable", common.ErrAuthDatabaseUnavailable)
	}

	const query = `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, created_at
	`

	var user User
	err := r.db.QueryRow(ctx, query, username, passwordHash).
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

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (UserCredentials, error) {
	if !r.dbStatus.AuthAvailable() {
		return UserCredentials{}, fmt.Errorf("%w: auth database is unavailable", common.ErrAuthDatabaseUnavailable)
	}

	const query = `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = $1
	`

	var credentials UserCredentials
	err := r.db.QueryRow(ctx, query, username).Scan(
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

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}
