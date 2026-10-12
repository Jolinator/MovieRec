package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"backend/internal/common"
	"backend/internal/database"
	"backend/internal/httpx"
	"backend/internal/platform"
)

const serviceHealthTimeout = 3 * time.Second

// newHealthHandler merges each service's own health into the single report the frontend and test harness read.
// A service that cannot be reached is reported as unavailable rather than failing the whole response.
func newHealthHandler(config Config) http.HandlerFunc {
	client := &http.Client{Timeout: serviceHealthTimeout}

	return func(w http.ResponseWriter, r *http.Request) {
		scanned, scanRequested, err := platform.ParseHealthScanQuery(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var auth, movies, recommendations common.ServiceHealth
		var running sync.WaitGroup
		for _, check := range []struct {
			upstream Upstream
			database database.Database
			result   *common.ServiceHealth
		}{
			{config.Auth, database.AuthDatabase, &auth},
			{config.Movies, database.MoviesDatabase, &movies},
			{config.Recommendations, database.RecommendationsDatabase, &recommendations},
		} {
			scan := ""
			if scanRequested {
				scan = fmt.Sprintf("?scan=%t", contains(scanned, check.database))
			}
			running.Add(1)
			go func() {
				defer running.Done()
				if health, err := fetchHealth(r.Context(), client, check.upstream.Internal.String()+common.InternalHealthPath+scan); err == nil {
					*check.result = health
				}
			}()
		}
		running.Wait()

		httpx.WriteJSON(w, common.HealthResponse{
			Message:                    "API is healthy!",
			BackendAvailable:           true,
			ImportedMoviesAvailable:    movies.Ready,
			AuthDBAvailable:            auth.DatabaseAvailable,
			MoviesDBAvailable:          movies.DatabaseAvailable,
			RecommendationsDBAvailable: recommendations.DatabaseAvailable,
			RecommendationsReady:       recommendations.Ready,
		})
	}
}

func fetchHealth(ctx context.Context, client *http.Client, url string) (common.ServiceHealth, error) {
	var health common.ServiceHealth
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return health, err
	}
	response, err := client.Do(request)
	if err != nil {
		return health, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return health, fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	err = json.NewDecoder(response.Body).Decode(&health)
	return health, err
}

func contains(databases []database.Database, target database.Database) bool {
	for _, candidate := range databases {
		if candidate == target {
			return true
		}
	}
	return false
}
