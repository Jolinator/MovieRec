package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/common"
)

func TestErrorMappings(t *testing.T) {
	boom := errors.New("secret internal detail")
	tests := []struct {
		name  string
		write func(http.ResponseWriter, error)
		err   error
		want  int
	}{
		{"signup conflict", writeSignupError, fmt.Errorf("w: %w", common.ErrUsernameAlreadyExists), 409},
		{"signup persistence", writeSignupError, common.ErrUserPersistence, 503},
		{"signup db", writeSignupError, common.ErrAuthDatabaseUnavailable, 503},
		{"signup other", writeSignupError, boom, 500},
		{"login invalid", writeLoginError, common.ErrInvalidCredentials, 401},
		{"login persistence", writeLoginError, common.ErrUserPersistence, 503},
		{"login db", writeLoginError, fmt.Errorf("w: %w", common.ErrAuthDatabaseUnavailable), 503},
		{"login other", writeLoginError, boom, 500},
		{"search db", writeUserSearchError, common.ErrAuthDatabaseUnavailable, 503},
		{"search persistence", writeUserSearchError, common.ErrUserPersistence, 503},
		{"search other", writeUserSearchError, boom, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.write(rec, tt.err)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Fatal("internal error leaked")
			}
		})
	}
}
