package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func AttemptConnect(databaseContext context.Context) (*pgxpool.Pool, error) {
	pool, err := openDatabaseWithRetry(databaseContext, 10, 2*time.Second)
	if err != nil {
		log.Printf("database startup failed: %v", err)
		return nil, err
	}
	return pool, nil
}

func setupServer(pool *pgxpool.Pool) *http.Server {
	server := &Server{db: pool}
	mux := http.NewServeMux()
	RegisterEndpoints(mux, server)
	handler := requestMetricsMiddleware(corsMiddleware(mux))

	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return httpServer
}

func RunServer(pool *pgxpool.Pool, httpServer *http.Server) chan error {

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- httpServer.ListenAndServe()
	}()

	log.Println("API running at http://localhost:8080")
	return serverErrors
}

func HandleErrors(serverErrors chan error, databaseContext context.Context, httpServer *http.Server) {
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server failed: %v", err)
		}
	case <-databaseContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownContext); err != nil {
			log.Printf("HTTP server shutdown failed: %v", err)
		}
	}
}

func main() {
	if err := loadLocalEnvironment(); err != nil {
		log.Printf("load local environment: %v", err)
		return
	}
	databaseContext, stopDatabaseSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopDatabaseSignal()

	pool, err := AttemptConnect(databaseContext)
	if err != nil {
		return
	}
	defer pool.Close()

	httpServer := setupServer(pool)

	serverErrors := RunServer(pool, httpServer)
	HandleErrors(serverErrors, databaseContext, httpServer)
}
