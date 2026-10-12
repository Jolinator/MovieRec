package movies

import (
	"errors"
	"net/http"

	"backend/internal/common"
	"backend/internal/httpx"
)

type Handler struct {
	service *MovieService
}

func NewHandler(service *MovieService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /movieAPI/movies", h.moviesHandler)
	mux.HandleFunc("GET /movieAPI/genres", h.genresHandler)
}

func (h *Handler) moviesHandler(w http.ResponseWriter, r *http.Request) {
	movieList, err := h.service.ListFromQuery(r.Context(), r.URL.Query())
	if err != nil {
		writeMoviesError(w, err, "could not load movies")
		return
	}
	httpx.WriteJSON(w, movieList)
}

func (h *Handler) genresHandler(w http.ResponseWriter, r *http.Request) {
	genres, err := h.service.ListGenres(r.Context())
	if err != nil {
		if errors.Is(err, common.ErrMoviesDatabaseUnavailable) {
			http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not load genres", http.StatusInternalServerError)
		return
	}
	httpx.WriteJSON(w, genres)
}

func writeMoviesError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, common.ErrInvalidMoviePagination):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, common.ErrMovieNotFound):
		http.Error(w, common.ErrMovieNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, common.ErrMoviesDatabaseUnavailable):
		http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}
