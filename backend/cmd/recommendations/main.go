// Command recommendations serves movie lists, personalised recommendations and the recommendation chat.
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
	"backend/internal/platform"
	"backend/internal/recommendations"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The movies service may still be importing from TMDB when this one starts, so initialisation keeps
// retrying until the catalogue is ready: 300 attempts at 2s is ten minutes.
const (
	initializeAttempts = 300
	initializeDelay    = 2 * time.Second
)

func main() {
	if err := database.LoadEnvironmentFiles("recommendations.env", "ports.env"); err != nil {
		log.Printf("load local environment: %v", err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.SetupPool(ctx, "RECOMMENDATIONS_")
	if err != nil {
		return
	}
	defer pool.Close()
	status := database.NewDBStatus(pool)
	go status.Start(ctx, 5*time.Second)

	tokenVerifier, err := platform.SetupTokenVerifier()
	if err != nil {
		log.Printf("JWT verifier startup failed: %v", err)
		return
	}

	port := platform.PortFromEnv("RECOMMENDATIONS_PORT", "8084")
	internalPort := platform.PortFromEnv("RECOMMENDATIONS_INTERNAL_PORT", "8085")
	if port == "" || internalPort == "" {
		return
	}

	moviesClient := recommendations.NewMoviesClient()
	listRepository := recommendations.NewListRepository(pool, status)
	cacheRepository := recommendations.NewRecommendationCacheRepository(pool, status)
	embeddingRepository := recommendations.NewEmbeddingRepository(pool, status)
	service := recommendations.NewService(moviesClient, embeddingRepository, listRepository, cacheRepository)
	go func() {
		if err := service.InitializeWithRetry(ctx, initializeAttempts, initializeDelay); err != nil {
			log.Printf("recommendation models unavailable: %v", err)
		}
	}()

	handler := recommendations.NewHandler(service, recommendations.NewListService(listRepository, moviesClient))
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	handler.RegisterRoutes(mux, platform.AuthenticationMiddleware(tokenVerifier))

	internalMux := http.NewServeMux()
	internalMux.HandleFunc("GET "+common.InternalHealthPath, platform.ServiceHealthHandler(status, service.Ready))
	handler.RegisterInternalRoutes(internalMux)

	publicServer := platform.NewHTTPServer(port, platform.RequestMetricsMiddleware(mux))
	internalServer := platform.NewHTTPServer(internalPort, internalMux)
	platform.HandleErrors(platform.RunServers(publicServer, internalServer), ctx, publicServer, internalServer)
}
