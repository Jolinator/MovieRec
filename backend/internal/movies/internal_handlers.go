package movies

import (
	"net/http"

	"backend/internal/common"
	"backend/internal/httpx"
)

const internalPrefix = "/internal"

// RegisterInternalRoutes registers the service-to-service API. It must never be exposed through the public gateway.
func (h *Handler) RegisterInternalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+internalPrefix+"/movies", h.moviesByIDsHandler)
	mux.HandleFunc("GET "+internalPrefix+"/movies/ready", h.moviesReadyHandler)
	mux.HandleFunc("GET "+internalPrefix+"/movies/synopses", h.movieSynopsesHandler)
}

func (h *Handler) moviesReadyHandler(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, common.ReadyResponse{Ready: h.service.Imported()})
}

func (h *Handler) moviesByIDsHandler(w http.ResponseWriter, r *http.Request) {
	movies, err := h.service.ListByTMDBIDsFromQuery(r.Context(), r.URL.Query())
	if err != nil {
		writeMoviesError(w, err, "could not load movies")
		return
	}
	httpx.WriteJSON(w, movies)
}

func (h *Handler) movieSynopsesHandler(w http.ResponseWriter, r *http.Request) {
	synopses, err := h.service.ListSynopsesFromQuery(r.Context(), r.URL.Query())
	if err != nil {
		writeMoviesError(w, err, "could not load movie synopses")
		return
	}
	httpx.WriteJSON(w, synopses)
}
