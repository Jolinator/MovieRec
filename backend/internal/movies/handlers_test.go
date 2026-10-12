package movies

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/common"
)

func TestWriteMoviesError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"invalid pagination", fmt.Errorf("%w: limit bad", common.ErrInvalidMoviePagination), http.StatusBadRequest, "limit bad"},
		{"not found", fmt.Errorf("wrap: %w", common.ErrMovieNotFound), http.StatusNotFound, common.ErrMovieNotFound.Error()},
		{"db unavailable", common.ErrMoviesDatabaseUnavailable, http.StatusServiceUnavailable, common.ErrMoviesDatabaseUnavailable.Error()},
		{"other", errors.New("secret internal detail"), http.StatusInternalServerError, "fallback message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeMoviesError(rec, tt.err, "fallback message")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q", body)
			}
			if tt.wantStatus == http.StatusInternalServerError && strings.Contains(body, "secret") {
				t.Fatal("internal error detail leaked to the client")
			}
		})
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	handler := NewHandler(NewMovieService(nil)) // validation errors never reach the repository
	handler.RegisterRoutes(mux)
	handler.RegisterInternalRoutes(mux)
	return mux
}

func TestHandlersRejectInvalidQueries(t *testing.T) {
	tests := []string{
		"/movieAPI/movies?limit=0",
		"/movieAPI/movies?limit=abc",
		"/movieAPI/movies?offset=-1",
		"/movieAPI/movies?genre=0",
		"/internal/movies",
		"/internal/movies?ids=abc",
		"/internal/movies?ids=0",
		"/internal/movies/synopses?limit=500",
		"/internal/movies/synopses?after=-1",
	}
	mux := newMux()
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestReadyHandlerReflectsImportState(t *testing.T) {
	service := NewMovieService(nil)
	mux := http.NewServeMux()
	NewHandler(service).RegisterInternalRoutes(mux)

	get := func() string {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/movies/ready", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if got := get(); got != `{"ready":false}` {
		t.Fatalf("before import: %s", got)
	}
	service.MarkImported()
	if got := get(); got != `{"ready":true}` {
		t.Fatalf("after import: %s", got)
	}
}

func TestRoutesAllowOnlyGET(t *testing.T) {
	mux := newMux()
	for _, target := range []string{"/movieAPI/movies", "/movieAPI/genres", "/internal/movies/ready"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", target, rec.Code)
		}
	}
}
