package platform

import (
	"errors"
	"net/http"
	"strings"

	"backend/internal/database"
)

func parseHealthScanQuery(r *http.Request) ([]database.Database, bool, error) {
	scan, requested := r.URL.Query()["scan"]
	if !requested {
		return nil, false, nil
	}
	if len(scan) != 1 || strings.TrimSpace(scan[0]) == "" {
		return nil, true, errors.New("scan must contain comma-separated database names")
	}

	var databases []database.Database
	for _, target := range strings.Split(scan[0], ",") {
		switch strings.TrimSpace(target) {
		case "auth":
			databases = append(databases, database.AuthDatabase)
		case "movies":
			databases = append(databases, database.MoviesDatabase)
		case "recommendations":
			databases = append(databases, database.RecommendationsDatabase)
		default:
			return nil, true, errors.New("unsupported health scan database: " + strings.TrimSpace(target))
		}
	}
	return databases, true, nil
}
