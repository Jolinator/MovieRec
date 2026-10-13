package recommendations

import (
	"errors"
	"net/http"

	"backend/internal/common"
	"backend/internal/httpx"
	"backend/internal/platform"

	"github.com/google/uuid"
)

type Handler struct {
	service *Service
	lists   *ListService
}

func NewHandler(service *Service, lists *ListService) *Handler {
	return &Handler{service: service, lists: lists}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authenticated func(http.Handler) http.Handler) {
	mux.Handle("GET /movieAPI/users/{userId}/list", authenticated(http.HandlerFunc(h.userListHandler)))
	mux.Handle("GET /movieAPI/my-list", authenticated(http.HandlerFunc(h.listHandler)))
	mux.Handle("POST /movieAPI/my-list", authenticated(http.HandlerFunc(h.addToListHandler)))
	mux.Handle("DELETE /movieAPI/my-list/{tmdbId}", authenticated(http.HandlerFunc(h.removeFromListHandler)))
	mux.Handle("POST /movieAPI/recommendations/chat", authenticated(http.HandlerFunc(h.recommendationChatHandler)))
	mux.Handle("GET /movieAPI/recommendations", authenticated(http.HandlerFunc(h.personalizedRecommendationsHandler)))
}

func (h *Handler) recommendationChatHandler(w http.ResponseWriter, r *http.Request) {
	request, ok := httpx.ValidateRequest[RecommendationChatRequest](r, w)
	if !ok {
		return
	}

	response, err := h.service.Chat(r.Context(), request.Message)
	if err != nil {
		if errors.Is(err, common.ErrRecommendationsUnavailable) ||
			errors.Is(err, common.ErrRecommendationsDatabaseUnavailable) ||
			errors.Is(err, common.ErrMoviesDatabaseUnavailable) ||
			errors.Is(err, common.ErrInferenceUnavailable) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not generate recommendations", http.StatusBadGateway)
		return
	}
	httpx.WriteJSON(w, response)
}

func (h *Handler) personalizedRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, common.ErrInvalidToken.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response, err := h.service.Recommendations(r.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrRecommendationsDatabaseUnavailable) ||
			errors.Is(err, common.ErrMoviesDatabaseUnavailable) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not load recommendations", http.StatusInternalServerError)
		return
	}
	httpx.WriteJSON(w, response)
}

func (h *Handler) userListHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		http.Error(w, "userId must be a valid UUID", http.StatusBadRequest)
		return
	}

	movies, err := h.lists.ListForUser(r.Context(), userID)
	if err != nil {
		writeMovieListError(w, err)
		return
	}
	httpx.WriteJSON(w, movies)
}

func (h *Handler) listHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	movies, err := h.lists.List(r.Context(), userID)
	if err != nil {
		writeMovieListError(w, err)
		return
	}
	httpx.WriteJSON(w, movies)
}

func (h *Handler) addToListHandler(w http.ResponseWriter, r *http.Request) {
	request, ok := httpx.ValidateRequest[AddMovieToListPostFormat](r, w)
	if !ok {
		return
	}
	userID, ok := platform.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	movie, err := h.lists.Add(r.Context(), userID, request.TMDBID)
	if err != nil {
		writeMovieListError(w, err)
		return
	}
	h.service.RequestRefresh(r.Context(), userID)
	w.WriteHeader(http.StatusCreated)
	httpx.WriteJSON(w, movie)
}

func (h *Handler) removeFromListHandler(w http.ResponseWriter, r *http.Request) {
	tmdbID, err := ParseTMDBID(r.PathValue("tmdbId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := platform.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	if err := h.lists.Remove(r.Context(), userID, tmdbID); err != nil {
		writeMovieListError(w, err)
		return
	}
	h.service.RequestRefresh(r.Context(), userID)
	httpx.WriteJSON(w, map[string]bool{"success": true})
}

func writeMovieListError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, common.ErrMovieNotFound):
		http.Error(w, common.ErrMovieNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, common.ErrMovieAlreadyInList):
		http.Error(w, common.ErrMovieAlreadyInList.Error(), http.StatusConflict)
	case errors.Is(err, common.ErrRecommendationsDatabaseUnavailable):
		http.Error(w, common.ErrRecommendationsDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrMoviesDatabaseUnavailable):
		http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, common.ErrMovieListPersistence):
		http.Error(w, common.ErrMovieListPersistence.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "could not update movie list", http.StatusInternalServerError)
	}
}
