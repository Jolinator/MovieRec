package gateway

import (
	"net/http"
	"os"
)

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
