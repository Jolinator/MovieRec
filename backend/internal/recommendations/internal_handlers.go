package recommendations

import (
	"net/http"

	"backend/internal/common"

	"github.com/google/uuid"
)

// RegisterInternalRoutes registers the service-to-service API used by the auth service. It must never be exposed through the public gateway.
func (h *Handler) RegisterInternalRoutes(mux *http.ServeMux) {
	base := "POST " + common.InternalRecommendationsUsersPath + "{userId}/"
	mux.HandleFunc(base+common.InternalEnsureFreshAction, h.ensureFreshHandler)
	mux.HandleFunc(base+common.InternalRequestRefreshAction, h.requestRefreshHandler)
}

func (h *Handler) ensureFreshHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := internalUserID(w, r)
	if !ok {
		return
	}
	h.service.EnsureFresh(r.Context(), userID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) requestRefreshHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := internalUserID(w, r)
	if !ok {
		return
	}
	h.service.RequestRefresh(r.Context(), userID)
	w.WriteHeader(http.StatusNoContent)
}

func internalUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		http.Error(w, "userId must be a valid UUID", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return userID, true
}
