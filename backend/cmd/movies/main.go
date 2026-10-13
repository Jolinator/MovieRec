// Command movies serves the movie catalogue and imports it from TMDB.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/common"
	"backend/internal/database"
	"backend/internal/movies"
	"backend/internal/platform"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := database.LoadEnvironmentFiles("movies.env", "tmdb.env", "ports.env"); err != nil {
		log.Printf("load local environment: %v", err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.SetupPool(ctx, "MOVIES_")
	if err != nil {
		return
	}
	defer pool.Close()
	status := database.NewDBStatus(pool, "movies")
	go status.Start(ctx, 5*time.Second)

	port := platform.PortFromEnv("MOVIES_PORT", "8083")
	internalPort := platform.PortFromEnv("MOVIES_INTERNAL_PORT", "8081")
	if port == "" || internalPort == "" {
		return
	}

	repository := movies.NewMovieRepository(pool, status)
	service := movies.NewMovieService(repository)
	go func() {
		if err := movies.ImportPopularMoviesWithRetry(ctx, repository, 10, 2*time.Second); err != nil {
			log.Printf("movie import failed: %v", err)
			return
		}
		service.MarkImported()
	}()

	handler := movies.NewHandler(service)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	handler.RegisterRoutes(mux)

	internalMux := http.NewServeMux()
	internalMux.HandleFunc("GET "+common.InternalHealthPath, platform.ServiceHealthHandler(status, service.Imported))
	handler.RegisterInternalRoutes(internalMux)

	publicServer := platform.NewHTTPServer(port, platform.RequestMetricsMiddleware(mux, "movies"))
	internalServer := platform.NewHTTPServer(internalPort, internalMux)
	platform.HandleErrors(platform.RunServers(publicServer, internalServer), ctx, publicServer, internalServer)
}
