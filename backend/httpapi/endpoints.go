package httpapi

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"backend/auth"
	"backend/common"
	"backend/database"
	"backend/movies"
	"backend/recommendations"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	dbStatuses            database.Statuses
	importedMovies        *atomic.Bool
	userService           *auth.UserService
	movieService          *movies.MovieService
	listService           *recommendations.ListService
	tokenVerifier         *auth.TokenVerifier
	recommendationService *recommendations.Service
}

func NewServer(dbStatuses database.Statuses, importedMovies *atomic.Bool, userService *auth.UserService, movieService *movies.MovieService, listService *recommendations.ListService, tokenVerifier *auth.TokenVerifier, recommendationService *recommendations.Service) *Server {
	return &Server{
		dbStatuses:            dbStatuses,
		importedMovies:        importedMovies,
		userService:           userService,
		movieService:          movieService,
		listService:           listService,
		tokenVerifier:         tokenVerifier,
		recommendationService: recommendationService,
	}
}

func RegisterEndpoints(mux *http.ServeMux, server *Server) {
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /movieAPI/{$}", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/health", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/movies", server.moviesHandler)
	mux.HandleFunc("GET /movieAPI/genres", server.genresHandler)
	mux.Handle("GET /movieAPI/users", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.usersHandler)))
	mux.Handle("GET /movieAPI/users/{userId}/list", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.userListHandler)))
	mux.Handle("GET /movieAPI/my-list", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.listHandler)))
	mux.Handle("POST /movieAPI/my-list", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.addToListHandler)))
	mux.Handle("DELETE /movieAPI/my-list/{tmdbId}", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.removeFromListHandler)))
	mux.HandleFunc("POST /movieAPI/login", server.loginHandler)
	mux.HandleFunc("POST /movieAPI/logout", server.logoutHandler)
	mux.Handle("GET /movieAPI/session", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.sessionHandler)))
	mux.HandleFunc("POST /movieAPI/signup", server.signUpHandler)
	mux.Handle("POST /movieAPI/recommendations/chat", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.recommendationChatHandler)))
	mux.Handle("GET /movieAPI/recommendations", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.personalizedRecommendationsHandler)))
	mux.HandleFunc("GET /movieAPI/recommendations/ready", server.recommendationReadyHandler)
	mux.HandleFunc("/", redirectToMovieAPI)
}

func (s *Server) recommendationReadyHandler(w http.ResponseWriter, r *http.Request) {
	common.WriteJSON(w, common.RecommendationReadyResponse{Ready: s.recommendationService.Ready()})
}

func (s *Server) recommendationChatHandler(w http.ResponseWriter, r *http.Request) {
	request, ok := common.ValidateRequest[common.RecommendationChatRequest](r, w)
	if !ok {
		return
	}

	response, err := s.recommendationService.Chat(r.Context(), request.Message)
	if err != nil {
		if errors.Is(err, common.ErrRecommendationsUnavailable) ||
			errors.Is(err, common.ErrMoviesDatabaseUnavailable) ||
			errors.Is(err, common.ErrInferenceUnavailable) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not generate recommendations", http.StatusBadGateway)
		return
	}
	common.WriteJSON(w, response)
}

func (s *Server) personalizedRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, common.ErrInvalidToken.Error(), http.StatusUnauthorized)
		return
	}
	response, err := s.recommendationService.Recommendations(r.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrRecommendationsDatabaseUnavailable) ||
			errors.Is(err, common.ErrMoviesDatabaseUnavailable) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not load recommendations", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, response)
}

func CorsMiddleware(next http.Handler) http.Handler {
	frontendPort := os.Getenv("FRONTEND_PORT")
	if frontendPort == "" {
		frontendPort = "5173"
	}
	frontendOrigin := "http://localhost:" + frontendPort
	secureFrontendOrigin := "https://localhost:" + frontendPort

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == frontendOrigin || origin == secureFrontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func redirectToMovieAPI(w http.ResponseWriter, r *http.Request) {

	if strings.HasPrefix(r.URL.Path, "/movieAPI") {
		http.NotFound(w, r)
		return
	}

	path := "/movieAPI" + r.URL.Path
	http.Redirect(w, r, path, http.StatusTemporaryRedirect)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	databases, scanRequested, err := parseHealthScanQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if scanRequested {
		s.dbStatuses.Scan(r.Context(), databases...)
	} else {
		s.dbStatuses.Refresh(r.Context())
	}

	response := common.HealthResponse{
		Message:                    "API is healthy!",
		BackendAvailable:           true,
		ImportedMoviesAvailable:    s.importedMovies.Load(),
		AuthDBAvailable:            s.dbStatuses[database.AuthDatabase].Available(),
		MoviesDBAvailable:          s.dbStatuses[database.MoviesDatabase].Available(),
		RecommendationsDBAvailable: s.dbStatuses[database.RecommendationsDatabase].Available(),
		RecommendationsReady:       s.recommendationService.Ready(),
	}
	common.WriteJSON(w, response)
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {

	request, ok := common.ValidateRequest[common.LoginPostFormat](r, w)
	if !ok {
		return
	}

	user, token, err := s.userService.Authenticate(r.Context(), request.UserName, request.Password)
	if err != nil {
		writeLoginError(w, err)
		return
	}

	auth.SetAccessTokenCookie(w, token)
	s.recommendationService.EnsureFresh(r.Context(), user.ID)
	common.WriteJSON(w, common.LoginResponse{LoggedIn: true, UserID: user.ID.String()})
}

func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	user, err := s.userService.FindByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		http.Error(w, "could not load session", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, common.SessionResponse{
		LoggedIn: true,
		UserID:   user.ID.String(),
		Username: user.Username,
	})
}

func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		s.recommendationService.RequestRefresh(r.Context(), userID)
	} else if cookie, err := r.Cookie("access_token"); err == nil {
		if userID, err := s.tokenVerifier.Verify(cookie.Value); err == nil {
			s.recommendationService.RequestRefresh(r.Context(), userID)
		}
	}
	auth.ClearAccessTokenCookie(w)
	common.WriteJSON(w, map[string]bool{"success": true})
}

func (s *Server) signUpHandler(w http.ResponseWriter, r *http.Request) {

	request, ok := common.ValidateRequest[common.SignUpPostFormat](r, w)
	if !ok {
		return
	}

	user, err := s.userService.Register(r.Context(), request.UserName, request.Password)
	if err != nil {
		writeSignupError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	common.WriteJSON(w, common.SignUpResponse{Success: true, UserID: user.ID.String()})
}

func (s *Server) moviesHandler(w http.ResponseWriter, r *http.Request) {
	movieList, err := s.movieService.ListFromQuery(r.Context(), r.URL.Query())
	if err != nil {
		if errors.Is(err, common.ErrInvalidMoviePagination) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, common.ErrMoviesDatabaseUnavailable) {
			http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not load movies", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, movieList)
}

func (s *Server) genresHandler(w http.ResponseWriter, r *http.Request) {
	genres, err := s.movieService.ListGenres(r.Context())
	if err != nil {
		if errors.Is(err, common.ErrMoviesDatabaseUnavailable) {
			http.Error(w, common.ErrMoviesDatabaseUnavailable.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "could not load genres", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, genres)
}

func (s *Server) usersHandler(w http.ResponseWriter, r *http.Request) {
	const maxUserSearchResults = 20

	users, err := s.userService.SearchUsers(r.Context(), r.URL.Query().Get("search"), maxUserSearchResults)
	if err != nil {
		writeUserSearchError(w, err)
		return
	}

	response := make([]common.UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, common.UserResponse{
			ID:       user.ID.String(),
			Username: user.Username,
		})
	}
	common.WriteJSON(w, response)
}

func (s *Server) userListHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		http.Error(w, "userId must be a valid UUID", http.StatusBadRequest)
		return
	}

	movies, err := s.listService.ListForUser(r.Context(), userID)
	if err != nil {
		writeMovieListError(w, err)
		return
	}
	common.WriteJSON(w, movies)
}

func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	movies, err := s.listService.List(r.Context(), userID)
	if err != nil {
		writeMovieListError(w, err)
		return
	}
	common.WriteJSON(w, movies)
}

func (s *Server) addToListHandler(w http.ResponseWriter, r *http.Request) {
	request, ok := common.ValidateRequest[common.AddMovieToListPostFormat](r, w)
	if !ok {
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	movie, err := s.listService.Add(r.Context(), userID, request.TMDBID)
	if err != nil {
		writeMovieListError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	common.WriteJSON(w, movie)
}

func (s *Server) removeFromListHandler(w http.ResponseWriter, r *http.Request) {
	tmdbID, err := recommendations.ParseTMDBID(r.PathValue("tmdbId"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	if err := s.listService.Remove(r.Context(), userID, tmdbID); err != nil {
		writeMovieListError(w, err)
		return
	}
	common.WriteJSON(w, map[string]bool{"success": true})
}
