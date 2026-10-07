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

func AttemptConnect(databaseContext context.Context, prefix string) (*pgxpool.Pool, error) {
	pool, err := openDatabaseWithRetry(databaseContext, prefix, 10, 2*time.Second)
	if err != nil {
		log.Printf("%sdatabase startup failed: %v", prefix, err)
		return nil, err
	}
	return pool, nil
}

func setupServer(authDB *pgxpool.Pool, moviesDB *pgxpool.Pool) *http.Server {
	server := &Server{
		authDB:       authDB,
		moviesDB:     moviesDB,
		userService:  NewUserService(NewUserRepository(authDB)),
		movieService: NewMovieService(NewMovieRepository(moviesDB)),
	}
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

	pool, err := AttemptConnect(databaseContext, "AUTH_")
	if err != nil {
		return
	}
	defer pool.Close()

	moviesPool, err := AttemptConnect(databaseContext, "MOVIES_")
	if err != nil {
		return
	}
	defer moviesPool.Close()

	if err := importPopularMovies(databaseContext, NewMovieRepository(moviesPool)); err != nil {
		log.Printf("movie import failed: %v", err)
		return
	}

	httpServer := setupServer(pool, moviesPool)

	serverErrors := RunServer(pool, httpServer)
	HandleErrors(serverErrors, databaseContext, httpServer)
}
