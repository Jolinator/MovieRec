package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"backend/auth"
	"backend/database"
	"backend/httpapi"
	"backend/movies"
	"backend/recommendations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupServer(authDB *pgxpool.Pool, ctx context.Context, moviesDB, recommendationsDB *pgxpool.Pool, tokenIssuer *auth.TokenIssuer, tokenVerifier *auth.TokenVerifier, dbStatus *database.DBStatus, port string) *http.Server {

	movieRepository := movies.NewMovieRepository(moviesDB, dbStatus)
	go func() {
		if err := movies.ImportPopularMoviesWithRetry(ctx, movieRepository, 10, 2*time.Second); err != nil {
			log.Printf("movie import failed: %v", err)
		}
	}()
	userService := auth.NewUserService(auth.NewUserRepository(authDB, dbStatus), tokenIssuer)
	movieService := movies.NewMovieService(movieRepository)
	listRepository := recommendations.NewListRepository(recommendationsDB, dbStatus)
	listService := recommendations.NewListService(listRepository, movieRepository)
	server := httpapi.NewServer(dbStatus, userService, movieService, listService, tokenVerifier)
	mux := http.NewServeMux()
	httpapi.RegisterEndpoints(mux, server)
	handler := httpapi.RequestMetricsMiddleware(httpapi.CorsMiddleware(mux))

	httpServer := &http.Server{
		Addr:              ":" + port,
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

	log.Printf("API running at http://localhost%s", httpServer.Addr)
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

func set_port() string {
	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		log.Printf("invalid BACKEND_PORT %q", port)
		return ""
	}
	return port
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

	recommendationsPool, err := database.SetupPool(databaseContext, "RECOMMENDATIONS_")
	if err != nil {
		return
	}
	defer recommendationsPool.Close()

	dbStatus := database.NewDBStatus(authPool, moviesPool, recommendationsPool)
	go dbStatus.Start(databaseContext, 5*time.Second)

	tokenIssuer, err := auth.SetupTokenIssuer()
	if err != nil {
		log.Printf("JWT startup failed: %v", err)
		return
	}
	tokenVerifier, err := auth.SetupTokenVerifier()
	if err != nil {
		log.Printf("JWT verifier startup failed: %v", err)
		return
	}

	port := set_port()
	if port == "" {
		return
	}

	httpServer := setupServer(authPool, databaseContext, moviesPool, recommendationsPool, tokenIssuer, tokenVerifier, dbStatus, port)

	serverErrors := RunServer(authPool, httpServer)

	HandleErrors(serverErrors, databaseContext, httpServer)
}
