package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	authDB      *pgxpool.Pool
	moviesDB    *pgxpool.Pool
	userService *UserService
}

func RegisterEndpoints(mux *http.ServeMux, server *Server) {
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /movieAPI/{$}", server.healthHandler)
	mux.HandleFunc("GET /movieAPI/health", server.healthHandler)
	mux.HandleFunc("POST /movieAPI/login", server.loginHandler)
	mux.HandleFunc("POST /movieAPI/signup", server.signUpHandler)
	mux.HandleFunc("/", redirectToMovieAPI)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.authDB.Ping(ctx); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := s.moviesDB.Ping(ctx); err != nil {
		http.Error(w, "movie database unavailable", http.StatusServiceUnavailable)
		return
	}

	response := HealthResponse{Message: "API is healthy!"}
	writeJSON(w, response)
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {

	request, return_ := validateRequest[LoginPostFormat](r, w)
	if return_ == false {
		return
	}

	// Replace this example response with credential verification later.
	response := LoginResponse{LoggedIn: true, UserID: request.UserName} //for now send username back as user ID but will be different
	writeJSON(w, response)
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


