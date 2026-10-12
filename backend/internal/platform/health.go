package platform

import (
	"net/http"

	"backend/internal/common"
	"backend/internal/database"
	"backend/internal/httpx"
)

// HealthSources are the readiness signals the health endpoint reports on.
type HealthSources struct {
	Statuses             database.Statuses
	MoviesImported       func() bool
	RecommendationsReady func() bool
}

func HealthHandler(sources HealthSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		databases, scanRequested, err := parseHealthScanQuery(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if scanRequested {
			sources.Statuses.Scan(r.Context(), databases...)
		} else {
			sources.Statuses.Refresh(r.Context())
		}

		httpx.WriteJSON(w, common.HealthResponse{
			Message:                    "API is healthy!",
			BackendAvailable:           true,
			ImportedMoviesAvailable:    sources.MoviesImported(),
			AuthDBAvailable:            sources.Statuses[database.AuthDatabase].Available(),
			MoviesDBAvailable:          sources.Statuses[database.MoviesDatabase].Available(),
			RecommendationsDBAvailable: sources.Statuses[database.RecommendationsDatabase].Available(),
			RecommendationsReady:       sources.RecommendationsReady(),
		})
	}
}
