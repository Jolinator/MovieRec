// Command auth serves users, sessions and token issuing.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/auth"
	"backend/internal/common"
	"backend/internal/database"
	"backend/internal/platform"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := database.LoadEnvironmentFiles("auth.env", "ports.env"); err != nil {
		log.Printf("load local environment: %v", err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.SetupPool(ctx, "AUTH_")
	if err != nil {
		return
	}
	defer pool.Close()
	status := database.NewDBStatus(pool)
	go status.Start(ctx, 5*time.Second)

	tokenIssuer, err := auth.SetupTokenIssuer()
	if err != nil {
		log.Printf("JWT startup failed: %v", err)
		return
	}
	tokenVerifier, err := platform.SetupTokenVerifier()
	if err != nil {
		log.Printf("JWT verifier startup failed: %v", err)
		return
	}

	port := platform.PortFromEnv("AUTH_PORT", "8082")
	internalPort := platform.PortFromEnv("AUTH_INTERNAL_PORT", "8086")
	if port == "" || internalPort == "" {
		return
	}

	userService := auth.NewUserService(auth.NewUserRepository(pool, status), tokenIssuer)
	handler := auth.NewHandler(userService, tokenVerifier, auth.NewRecommendationsClient())

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	handler.RegisterRoutes(mux, platform.AuthenticationMiddleware(tokenVerifier))

	internalMux := http.NewServeMux()
	internalMux.HandleFunc("GET "+common.InternalHealthPath, platform.ServiceHealthHandler(status, func() bool { return true }))

	publicServer := platform.NewHTTPServer(port, platform.RequestMetricsMiddleware(mux))
	internalServer := platform.NewHTTPServer(internalPort, internalMux)
	platform.HandleErrors(platform.RunServers(publicServer, internalServer), ctx, publicServer, internalServer)
}
