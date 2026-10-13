// Command gateway is the public entrypoint: it routes /movieAPI requests to the auth, movies and
// recommendations services and serves the merged health report.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"backend/internal/database"
	"backend/internal/gateway"
	"backend/internal/platform"
)

func main() {
	if err := database.LoadEnvironmentFiles("ports.env"); err != nil {
		log.Printf("load local environment: %v", err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := gateway.LoadConfig()
	if err != nil {
		log.Printf("gateway configuration: %v", err)
		return
	}
	port := platform.PortFromEnv("BACKEND_PORT", "8080")
	if port == "" {
		return
	}

	mux := http.NewServeMux()
	gateway.RegisterRoutes(mux, config)
	server := platform.NewHTTPServer(port, platform.RequestMetricsMiddleware(gateway.CorsMiddleware(mux), "gateway"))

	platform.HandleErrors(platform.RunServers(server), ctx, server)
}
