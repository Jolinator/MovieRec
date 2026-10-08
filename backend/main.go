package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/auth"
	"backend/database"
	"backend/httpapi"
	"backend/movies"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupServer(authDB *pgxpool.Pool, ctx context.Context, moviesDB *pgxpool.Pool, tokenIssuer *auth.TokenIssuer, dbStatus *database.DBStatus) *http.Server {

	movieRepository := movies.NewMovieRepository(moviesDB, dbStatus)
	userService := auth.NewUserService(auth.NewUserRepository(authDB, dbStatus), tokenIssuer)
	movieService := movies.NewMovieService(movieRepository)
	server := httpapi.NewServer(dbStatus, userService, movieService)
	mux := http.NewServeMux()
	httpapi.RegisterEndpoints(mux, server)
	handler := httpapi.RequestMetricsMiddleware(httpapi.CorsMiddleware(mux))

	if err := movies.ImportPopularMoviesWithRetry(ctx, movieRepository, 10, 2*time.Second); err != nil {
		log.Printf("movie import failed: %v", err)
	}

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
	if err := database.LoadLocalEnvironment(); err != nil {
		log.Printf("load local environment: %v", err)
		return
	}
	databaseContext, stopDatabaseSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopDatabaseSignal()

	authPool, err := database.SetupPool(databaseContext, "AUTH_")
	if err != nil {
		return
	}
	defer authPool.Close()

	moviesPool, err := database.SetupPool(databaseContext, "MOVIES_")
	if err != nil {
		return
	}
	defer moviesPool.Close()

	dbStatus := database.NewDBStatus(authPool, moviesPool)
	go dbStatus.Start(databaseContext, 5*time.Second)

	tokenIssuer, err := auth.SetupTokenIssuer()
	if err != nil {
		log.Printf("JWT startup failed: %v", err)
		return
	}
	httpServer := setupServer(authPool, databaseContext, moviesPool, tokenIssuer, dbStatus)

	serverErrors := RunServer(authPool, httpServer)

	HandleErrors(serverErrors, databaseContext, httpServer)
}
