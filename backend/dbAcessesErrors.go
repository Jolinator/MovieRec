package main

import (
	"errors"
	"net/http"
)

func writeSignupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUsernameAlreadyExists):
		http.Error(w, "username already exists", http.StatusConflict)
	case errors.Is(err, ErrUserPersistence):
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not create user", http.StatusInternalServerError)
	}
}

func writeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		http.Error(w, "incorrect username or password", http.StatusUnauthorized)
	case errors.Is(err, ErrUserPersistence):
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not log in", http.StatusInternalServerError)
	}
}
