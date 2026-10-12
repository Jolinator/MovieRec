package httpapi

import (
	"net/http"

	"backend/common"
)

const internalPrefix = "/internal"

// RegisterInternalEndpoints registers the service-to-service API. It must never be exposed through the public gateway.
func RegisterInternalEndpoints(mux *http.ServeMux, server *Server) {
	mux.HandleFunc("GET "+internalPrefix+"/movies", server.moviesByIDsHandler)
	mux.HandleFunc("GET "+internalPrefix+"/movies/synopses", server.movieSynopsesHandler)
}

func (s *Server) moviesByIDsHandler(w http.ResponseWriter, r *http.Request) {
	movies, err := s.movieService.ListByTMDBIDsFromQuery(r.Context(), r.URL.Query())
	if err != nil {
		writeMoviesError(w, err, "could not load movies")
		return
	}
	common.WriteJSON(w, movies)
}

func (s *Server) movieSynopsesHandler(w http.ResponseWriter, r *http.Request) {
	synopses, err := s.movieService.ListSynopsesFromQuery(r.Context(), r.URL.Query())
	if err != nil {
		writeMoviesError(w, err, "could not load movie synopses")
		return
	}
	common.WriteJSON(w, synopses)
}
