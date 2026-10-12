package auth

import (
	"context"
	"errors"
	"net/http"

	"backend/internal/common"
	"backend/internal/httpx"
	"backend/internal/platform"

	"github.com/google/uuid"
)

// RecommendationRefresher lets login and logout nudge the recommendations service without depending on it.
type RecommendationRefresher interface {
	EnsureFresh(ctx context.Context, userID uuid.UUID)
	RequestRefresh(ctx context.Context, userID uuid.UUID)
}

type Handler struct {
	users           *UserService
	verifier        *platform.TokenVerifier
	recommendations RecommendationRefresher
}

func NewHandler(users *UserService, verifier *platform.TokenVerifier, recommendations RecommendationRefresher) *Handler {
	return &Handler{users: users, verifier: verifier, recommendations: recommendations}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authenticated func(http.Handler) http.Handler) {
	mux.Handle("GET /movieAPI/users", authenticated(http.HandlerFunc(h.usersHandler)))
	mux.HandleFunc("POST /movieAPI/login", h.loginHandler)
	mux.HandleFunc("POST /movieAPI/logout", h.logoutHandler)
	mux.Handle("GET /movieAPI/session", authenticated(http.HandlerFunc(h.sessionHandler)))
	mux.HandleFunc("POST /movieAPI/signup", h.signUpHandler)
}

func (h *Handler) loginHandler(w http.ResponseWriter, r *http.Request) {

	request, ok := httpx.ValidateRequest[LoginPostFormat](r, w)
	if !ok {
		return
	}

	user, token, err := h.users.Authenticate(r.Context(), request.UserName, request.Password)
	if err != nil {
		writeLoginError(w, err)
		return
	}

	SetAccessTokenCookie(w, token)
	h.recommendations.EnsureFresh(r.Context(), user.ID)
	httpx.WriteJSON(w, LoginResponse{LoggedIn: true, UserID: user.ID.String()})
}

func (h *Handler) sessionHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		http.Error(w, "could not load session", http.StatusInternalServerError)
		return
	}
	httpx.WriteJSON(w, SessionResponse{
		LoggedIn: true,
		UserID:   user.ID.String(),
		Username: user.Username,
	})
}

func (h *Handler) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if userID, ok := platform.UserIDFromContext(r.Context()); ok {
		h.recommendations.RequestRefresh(r.Context(), userID)
	} else if cookie, err := r.Cookie(platform.AccessTokenCookie); err == nil {
		if userID, err := h.verifier.Verify(cookie.Value); err == nil {
			h.recommendations.RequestRefresh(r.Context(), userID)
		}
	}
	ClearAccessTokenCookie(w)
	httpx.WriteJSON(w, map[string]bool{"success": true})
}

func (h *Handler) signUpHandler(w http.ResponseWriter, r *http.Request) {

	request, ok := httpx.ValidateRequest[SignUpPostFormat](r, w)
	if !ok {
		return
	}

	user, err := h.users.Register(r.Context(), request.UserName, request.Password)
	if err != nil {
		writeSignupError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	httpx.WriteJSON(w, SignUpResponse{Success: true, UserID: user.ID.String()})
}

func (h *Handler) usersHandler(w http.ResponseWriter, r *http.Request) {
	const maxUserSearchResults = 20

	users, err := h.users.SearchUsers(r.Context(), r.URL.Query().Get("search"), maxUserSearchResults)
	if err != nil {
		writeUserSearchError(w, err)
		return
	}

	response := make([]UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, UserResponse{
			ID:       user.ID.String(),
			Username: user.Username,
		})
	}
	httpx.WriteJSON(w, response)
}
