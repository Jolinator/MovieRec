package gateway

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterRoutes exposes the public API by explicit route. The services' /internal APIs are deliberately
// not listed, so they can never be reached through the gateway.
func RegisterRoutes(mux *http.ServeMux, config Config) {
	auth := newProxy(config.Auth.Public)
	movies := newProxy(config.Movies.Public)
	recommendations := newProxy(config.Recommendations.Public)

	routes := []struct {
		pattern string
		handler http.Handler
	}{
		{"GET /movieAPI/users", auth},
		{"POST /movieAPI/login", auth},
		{"POST /movieAPI/logout", auth},
		{"GET /movieAPI/session", auth},
		{"POST /movieAPI/signup", auth},
		{"GET /movieAPI/movies", movies},
		{"GET /movieAPI/genres", movies},
		{"GET /movieAPI/users/{userId}/list", recommendations},
		{"GET /movieAPI/my-list", recommendations},
		{"POST /movieAPI/my-list", recommendations},
		{"DELETE /movieAPI/my-list/{tmdbId}", recommendations},
		{"POST /movieAPI/recommendations/chat", recommendations},
		{"GET /movieAPI/recommendations", recommendations},
	}
	for _, route := range routes {
		mux.Handle(route.pattern, route.handler)
	}

	health := newHealthHandler(config)
	mux.HandleFunc("GET /movieAPI/{$}", health)
	mux.HandleFunc("GET /movieAPI/health", health)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("/", RedirectToMovieAPI)
}
