package main

import (
	"errors"
	"net/http"
)

var (
	ErrUsernameAlreadyExists   = errors.New("username already exists")
	ErrUserNotFound            = errors.New("user not found")
	ErrUserPersistence         = errors.New("user persistence failed")
	ErrAuthDatabaseUnavailable = errors.New("auth database unavailable")
)

var (
	ErrInvalidMoviePagination    = errors.New("invalid movie pagination")
	ErrMoviesDatabaseUnavailable = errors.New("movies database unavailable")
)

func writeSignupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUsernameAlreadyExists):
		http.Error(w, ErrUsernameAlreadyExists.Error(), http.StatusConflict)
	case errors.Is(err, ErrUserPersistence):
		http.Error(w, ErrUserPersistence.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, ErrAuthDatabaseUnavailable):
		http.Error(w, ErrAuthDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not create user", http.StatusInternalServerError)
	}
}

func writeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		http.Error(w, ErrInvalidCredentials.Error(), http.StatusUnauthorized)
	case errors.Is(err, ErrUserPersistence):
		http.Error(w, ErrUserPersistence.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, ErrAuthDatabaseUnavailable):
		http.Error(w, ErrAuthDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not log in", http.StatusInternalServerError)
	}
}
