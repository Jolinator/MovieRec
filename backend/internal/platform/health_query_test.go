package platform

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"backend/internal/database"
)

func TestParseHealthScanQuery(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		wantDatabases []database.Database
		wantRequested bool
		wantError     bool
	}{
		{
			name: "scan not requested",
			url:  "/movieAPI/health",
		},
		{
			name:          "all database targets",
			url:           "/movieAPI/health?scan=auth,movies,recommendations",
			wantDatabases: []database.Database{database.AuthDatabase, database.MoviesDatabase, database.RecommendationsDatabase},
			wantRequested: true,
		},
		{
			name:          "empty scan",
			url:           "/movieAPI/health?scan=",
			wantRequested: true,
			wantError:     true,
		},
		{
			name:          "unsupported database",
			url:           "/movieAPI/health?scan=unknown",
			wantRequested: true,
			wantError:     true,
		},
		{
			name:          "repeated scan parameter",
			url:           "/movieAPI/health?scan=auth&scan=movies",
			wantRequested: true,
			wantError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", test.url, nil)
			databases, requested, err := ParseHealthScanQuery(request)

			if requested != test.wantRequested {
				t.Fatalf("requested = %t, want %t", requested, test.wantRequested)
			}
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %t", err, test.wantError)
			}
			if !reflect.DeepEqual(databases, test.wantDatabases) {
				t.Fatalf("databases = %v, want %v", databases, test.wantDatabases)
			}
		})
	}
}
