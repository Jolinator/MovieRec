package recommendations

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/common"
)

func TestInternalRefreshRejectsInvalidUserID(t *testing.T) {
	mux := http.NewServeMux()
	// The service is never reached because the user ID is rejected first.
	NewHandler(nil, nil).RegisterInternalRoutes(mux)

	for _, action := range []string{common.InternalEnsureFreshAction, common.InternalRequestRefreshAction} {
		request := httptest.NewRequest("POST", common.InternalRecommendationsUsersPath+"not-a-uuid/"+action, nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", action, recorder.Code)
		}
	}
}

func TestInternalRefreshIsPostOnly(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(nil, nil).RegisterInternalRoutes(mux)

	request := httptest.NewRequest("GET", common.InternalRecommendationsUsersPath+"6f1e8c1a-0000-0000-0000-000000000000/"+common.InternalEnsureFreshAction, nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
