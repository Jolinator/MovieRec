package httpapi

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"backend/auth"
	"backend/common"
	"backend/database"
	"backend/movies"
	"backend/recommendations"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	dbStatus      *database.DBStatus
	userService   *auth.UserService
	movieService  *movies.MovieService
	listService   *recommendations.ListService
	tokenVerifier *auth.TokenVerifier
}

func NewServer(dbStatus *database.DBStatus, userService *auth.UserService, movieService *movies.MovieService, listService *recommendations.ListService, tokenVerifier *auth.TokenVerifier) *Server {
	return &Server{dbStatus: dbStatus, userService: userService, movieService: movieService, listService: listService, tokenVerifier: tokenVerifier}
}

func RegisterEndpoints(mux *http.ServeMux, server *Server) {
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /movieAPI/{$}", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/health", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/movies", server.moviesHandler)
	mux.HandleFunc("GET /movieAPI/genres", server.genresHandler)
	mux.Handle("GET /movieAPI/my-list", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.listHandler)))
	mux.Handle("POST /movieAPI/my-list", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.addToListHandler)))
	mux.Handle("DELETE /movieAPI/my-list/{tmdbId}", AuthenticationMiddleware(server.tokenVerifier)(http.HandlerFunc(server.removeFromListHandler)))
	mux.HandleFunc("POST /movieAPI/login", server.loginHandler)
	mux.HandleFunc("POST /movieAPI/logout", server.logoutHandler)
	mux.HandleFunc("POST /movieAPI/signup", server.signUpHandler)
	mux.HandleFunc("/", redirectToMovieAPI)
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
	auth_available := s.dbStatus.AuthAvailable()
	movies_db_available := s.dbStatus.MoviesAvailable()
	recommendations_available := s.dbStatus.RecommendationsAvailable()
	imported_movies_available := s.dbStatus.ImportedMoviesAvailable()
	backend_available := true



	response := common.HealthResponse{Message: "API is healthy!", BackendAvailable: backend_available, ImportedMoviesAvailable: imported_movies_available, AuthDBAvailable: auth_available, MoviesDBAvailable: movies_db_available, RecommendationsDBAvailable: recommendations_available}
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
	common.WriteJSON(w, common.LoginResponse{LoggedIn: true, UserID: user.ID.String()})
}

func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
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
