package httpapi

import (
	"errors"
	"net/http"

	"backend/common"
)

func writeSignupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, common.ErrUsernameAlreadyExists):
		http.Error(w, common.ErrUsernameAlreadyExists.Error(), http.StatusConflict)
	case errors.Is(err, common.ErrUserPersistence):
		http.Error(w, common.ErrUserPersistence.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrAuthDatabaseUnavailable):
		http.Error(w, common.ErrAuthDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not create user", http.StatusInternalServerError)
	}
}

func writeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, common.ErrInvalidCredentials):
		http.Error(w, common.ErrInvalidCredentials.Error(), http.StatusUnauthorized)
	case errors.Is(err, common.ErrUserPersistence):
		http.Error(w, common.ErrUserPersistence.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrAuthDatabaseUnavailable):
		http.Error(w, common.ErrAuthDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not log in", http.StatusInternalServerError)
	}
}
