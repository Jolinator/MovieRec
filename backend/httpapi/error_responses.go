package httpapi

import (
	"errors"
	"net/http"

	"backend/common"
)

func writeMoviesError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, common.ErrInvalidMoviePagination):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, common.ErrMovieNotFound):
		http.Error(w, common.ErrMovieNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, common.ErrMoviesDatabaseUnavailable):
		http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeMovieListError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, common.ErrMovieNotFound):
		http.Error(w, common.ErrMovieNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, common.ErrMovieAlreadyInList):
		http.Error(w, common.ErrMovieAlreadyInList.Error(), http.StatusConflict)
	case errors.Is(err, common.ErrRecommendationsDatabaseUnavailable):
		http.Error(w, common.ErrRecommendationsDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrMoviesDatabaseUnavailable):
		http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrMovieListPersistence):
		http.Error(w, common.ErrMovieListPersistence.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not update movie list", http.StatusInternalServerError)
	}
}

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

func writeUserSearchError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, common.ErrAuthDatabaseUnavailable):
		http.Error(w, common.ErrAuthDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrUserPersistence):
		http.Error(w, common.ErrUserPersistence.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not search users", http.StatusInternalServerError)
	}
}
