package platform

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func NewHTTPServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func RunServers(httpServers ...*http.Server) chan error {
	serverErrors := make(chan error, len(httpServers))
	for _, httpServer := range httpServers {
		go func() {
			serverErrors <- httpServer.ListenAndServe()
		}()
		log.Printf("HTTP server running at http://localhost%s", httpServer.Addr)
	}
	return serverErrors
}

func HandleErrors(serverErrors chan error, databaseContext context.Context, httpServers ...*http.Server) {
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server failed: %v", err)
		}
	case <-databaseContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, httpServer := range httpServers {
			if err := httpServer.Shutdown(shutdownContext); err != nil {
				log.Printf("HTTP server shutdown failed: %v", err)
			}
		}
	}
}

// Env returns the named environment variable, or fallback when it is unset or empty.
func Env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// PortFromEnv returns the validated port in the named variable, or "" if it is invalid.
func PortFromEnv(name, fallback string) string {
	port := Env(name, fallback)
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		log.Printf("invalid %s %q", name, port)
		return ""
	}
	return port
}
