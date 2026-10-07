package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	authDB       *pgxpool.Pool
	moviesDB     *pgxpool.Pool
	dbStatus     *DBStatus
	userService  *UserService
	movieService *MovieService
}

func RegisterEndpoints(mux *http.ServeMux, server *Server) {
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /movieAPI/{$}", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/health", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/movies", server.moviesHandler)
	mux.HandleFunc("POST /movieAPI/login", server.loginHandler)
	mux.HandleFunc("POST /movieAPI/logout", server.logoutHandler)
	mux.HandleFunc("POST /movieAPI/signup", server.signUpHandler)
	mux.HandleFunc("/", redirectToMovieAPI)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "https://localhost:5173" {
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
	if !s.dbStatus.AuthAvailable() {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	if !s.dbStatus.MoviesAvailable() {
		http.Error(w, "movie database unavailable", http.StatusServiceUnavailable)
		return
	}

	response := HealthResponse{Message: "API is healthy!"}
	writeJSON(w, response)
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {

	request, ok := validateRequest[LoginPostFormat](r, w)
	if !ok {
		return
	}

	user, token, err := s.userService.Authenticate(r.Context(), request.UserName, request.Password)
	if err != nil {
		writeLoginError(w, err)
		return
	}

	setAccessTokenCookie(w, token)
	writeJSON(w, LoginResponse{LoggedIn: true, UserID: user.ID.String()})
}

func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	clearAccessTokenCookie(w)
	writeJSON(w, map[string]bool{"success": true})
}

func (s *Server) signUpHandler(w http.ResponseWriter, r *http.Request) {

	request, ok := validateRequest[SignUpPostFormat](r, w)
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
	writeJSON(w, SignUpResponse{Success: true, UserID: user.ID.String()})
}

func (s *Server) moviesHandler(w http.ResponseWriter, r *http.Request) {
    movies, err := s.movieService.ListFromQuery(r.Context(), r.URL.Query())
    if err != nil {
        if errors.Is(err, ErrInvalidMoviePagination) {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        http.Error(w, "could not load movies", http.StatusInternalServerError)
        return
    }
    writeJSON(w, movies)
}