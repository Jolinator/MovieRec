package recommendations

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/common"
	"backend/internal/platform"

	"github.com/google/uuid"
)

func passthrough(next http.Handler) http.Handler { return next }

func testMux(service *Service, lists *ListService) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandler(service, lists).RegisterRoutes(mux, passthrough)
	return mux
}

func do(mux *http.ServeMux, method, target, body string, userID *uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if userID != nil {
		req = req.WithContext(platform.WithUserID(req.Context(), *userID))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestWriteMovieListError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", common.ErrMovieNotFound, http.StatusNotFound},
		{"already in list", fmt.Errorf("x: %w", common.ErrMovieAlreadyInList), http.StatusConflict},
		{"recommendations db", common.ErrRecommendationsDatabaseUnavailable, http.StatusServiceUnavailable},
		{"movies db", common.ErrMoviesDatabaseUnavailable, http.StatusServiceUnavailable},
		{"persistence", common.ErrMovieListPersistence, http.StatusServiceUnavailable},
		{"other", errors.New("secret detail"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeMovieListError(rec, tt.err)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Fatal("internal error leaked")
			}
		})
	}
}

func TestReadyHandler(t *testing.T) {
	service := &Service{}
	mux := testMux(service, nil)
	if got := strings.TrimSpace(do(mux, "GET", "/movieAPI/recommendations/ready", "", nil).Body.String()); got != `{"ready":false}` {
		t.Fatalf("got %s", got)
	}
	service.ready.Store(true)
	if got := strings.TrimSpace(do(mux, "GET", "/movieAPI/recommendations/ready", "", nil).Body.String()); got != `{"ready":true}` {
		t.Fatalf("got %s", got)
	}
}

func TestUserListHandlerRejectsInvalidUUID(t *testing.T) {
	rec := do(testMux(nil, nil), "GET", "/movieAPI/users/not-a-uuid/list", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAuthenticatedHandlersRequireUserInContext(t *testing.T) {
	service := &Service{}
	service.ready.Store(true)
	mux := testMux(service, nil)
	tests := []struct {
		method, target, body string
	}{
		{"GET", "/movieAPI/my-list", ""},
		{"POST", "/movieAPI/my-list", `{"tmdbId":1}`},
		{"DELETE", "/movieAPI/my-list/1", ""},
		{"GET", "/movieAPI/recommendations", ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			if rec := do(mux, tt.method, tt.target, tt.body, nil); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestAddToListRejectsInvalidBody(t *testing.T) {
	id := uuid.New()
	mux := testMux(nil, nil)
	for _, body := range []string{``, `{`, `{"tmdbId":0}`, `{"tmdbId":-3}`, `{"tmdbId":"x"}`} {
		if rec := do(mux, "POST", "/movieAPI/my-list", body, &id); rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestRemoveFromListRejectsInvalidID(t *testing.T) {
	id := uuid.New()
	mux := testMux(nil, nil)
	for _, target := range []string{"abc", "0", "-1"} {
		if rec := do(mux, "DELETE", "/movieAPI/my-list/"+target, "", &id); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestAddToListSurfacesCatalogueErrors(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		err  error
		want int
	}{
		{common.ErrMovieNotFound, http.StatusNotFound},
		{common.ErrMoviesDatabaseUnavailable, http.StatusServiceUnavailable},
		{errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		lists := NewListService(nil, &fakeCatalog{listErr: tt.err})
		if rec := do(testMux(nil, lists), "POST", "/movieAPI/my-list", `{"tmdbId":5}`, &id); rec.Code != tt.want {
			t.Errorf("%v: status = %d, want %d", tt.err, rec.Code, tt.want)
		}
	}
}

func TestChatHandlerRejectsInvalidRequests(t *testing.T) {
	mux := testMux(&Service{}, nil)
	for _, body := range []string{``, `{`, `{"message":""}`, `{"message":"a"}`, `{"message":"` + strings.Repeat("a", 1001) + `"}`} {
		if rec := do(mux, "POST", "/movieAPI/recommendations/chat", body, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("body %.20q: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestChatHandlerReportsUnavailableWhenNotReady(t *testing.T) {
	rec := do(testMux(&Service{}, nil), "POST", "/movieAPI/recommendations/chat", `{"message":"funny movie"}`, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestChatHandlerMapsInferenceFailureTo503(t *testing.T) {
	embedding, generation := inferenceServer(t, false, true)
	service := &Service{embedding: embedding, generation: generation}
	service.ready.Store(true)
	rec := do(testMux(service, nil), "POST", "/movieAPI/recommendations/chat", `{"message":"funny movie"}`, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestRoutesRejectWrongMethods(t *testing.T) {
	mux := testMux(&Service{}, nil)
	for _, tt := range [][2]string{
		{"POST", "/movieAPI/recommendations"},
		{"GET", "/movieAPI/recommendations/chat"},
		{"PUT", "/movieAPI/my-list"},
	} {
		if rec := do(mux, tt[0], tt[1], "", nil); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", tt[0], tt[1], rec.Code)
		}
	}
}
