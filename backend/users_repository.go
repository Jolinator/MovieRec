package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrUserPersistence       = errors.New("user persistence failed")
)

type User struct {
	ID        uuid.UUID
	Username  string
	CreatedAt time.Time
}

type UserRepository struct {
	db       *pgxpool.Pool
	dbStatus *DBStatus
}

func NewUserRepository(db *pgxpool.Pool, dbStatus *DBStatus) *UserRepository {
	return &UserRepository{db: db, dbStatus: dbStatus}
}

func (r *UserRepository) Create(ctx context.Context, username, passwordHash string) (User, error) {
	if !r.dbStatus.AuthAvailable() {
		return User{}, fmt.Errorf("%w: auth database is unavailable", ErrUserPersistence)
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
		return User{}, ErrUsernameAlreadyExists
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("%w: insert returned no user", ErrUserPersistence)
	}

	return User{}, fmt.Errorf("%w: %v", ErrUserPersistence, err)
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}
