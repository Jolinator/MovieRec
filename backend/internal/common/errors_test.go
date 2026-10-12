package common

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrorsAreDistinct(t *testing.T) {
	errs := []error{
		ErrUsernameAlreadyExists, ErrUserNotFound, ErrUserPersistence, ErrAuthDatabaseUnavailable,
		ErrInvalidCredentials, ErrInvalidToken, ErrInvalidMoviePagination, ErrMoviesDatabaseUnavailable,
		ErrRecommendationsDatabaseUnavailable, ErrMovieNotFound, ErrMovieAlreadyInList,
		ErrMovieListPersistence, ErrInferenceUnavailable, ErrRecommendationsUnavailable,
	}
	for i, a := range errs {
		if a.Error() == "" {
			t.Errorf("error %d has empty message", i)
		}
		for j, b := range errs {
			if i != j && errors.Is(a, b) {
				t.Errorf("errors %d and %d match", i, j)
			}
		}
	}
}

func TestSentinelErrorsSurviveWrapping(t *testing.T) {
	if !errors.Is(fmt.Errorf("ctx: %w", ErrUserNotFound), ErrUserNotFound) {
		t.Fatal("wrapped error not matched")
	}
}
