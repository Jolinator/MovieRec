package common

import "errors"

var (
	ErrUsernameAlreadyExists     = errors.New("username already exists")
	ErrUserNotFound              = errors.New("user not found")
	ErrUserPersistence           = errors.New("user persistence failed")
	ErrAuthDatabaseUnavailable   = errors.New("auth database unavailable")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrInvalidMoviePagination    = errors.New("invalid movie pagination")
	ErrMoviesDatabaseUnavailable = errors.New("movies database unavailable")
)
