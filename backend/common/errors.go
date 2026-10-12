package common

import "errors"

var (
	ErrUsernameAlreadyExists              = errors.New("username already exists")
	ErrUserNotFound                       = errors.New("user not found")
	ErrUserPersistence                    = errors.New("user persistence failed")
	ErrAuthDatabaseUnavailable            = errors.New("auth database unavailable")
	ErrInvalidCredentials                 = errors.New("invalid credentials")
	ErrInvalidToken                       = errors.New("invalid authentication token")
	ErrInvalidMoviePagination             = errors.New("invalid movie pagination")
	ErrMoviesDatabaseUnavailable          = errors.New("movies database unavailable")
	ErrRecommendationsDatabaseUnavailable = errors.New("recommendations database unavailable")
	ErrMovieNotFound                      = errors.New("movie not found")
	ErrMovieAlreadyInList                 = errors.New("movie already in list")
	ErrMovieListPersistence               = errors.New("movie list persistence failed")
	ErrInferenceUnavailable               = errors.New("inference service unavailable")
	ErrRecommendationsUnavailable         = errors.New("recommendation service is temporarily unavailable; please wait and try again")
)
