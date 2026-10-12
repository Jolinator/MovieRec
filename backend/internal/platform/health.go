package platform

import (
	"net/http"

	"backend/internal/common"
	"backend/internal/database"
	"backend/internal/httpx"
)

// ServiceHealthHandler reports one service's own state. scan=true pings its database, scan=false
// reports the last known state, and no scan parameter rescans only if the database looks down.
func ServiceHealthHandler(status *database.DBStatus, ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("scan") {
		case "true":
			status.Scan(r.Context())
		case "false":
		default:
			status.Refresh(r.Context())
		}
		httpx.WriteJSON(w, common.ServiceHealth{DatabaseAvailable: status.Available(), Ready: ready()})
	}
}
