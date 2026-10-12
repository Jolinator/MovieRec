package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"backend/auth"
	"backend/database"
	"backend/httpapi"
	"backend/movies"
	"backend/recommendations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupServer(authDB *pgxpool.Pool, ctx context.Context, moviesDB, recommendationsDB *pgxpool.Pool, tokenIssuer *auth.TokenIssuer, tokenVerifier *auth.TokenVerifier, dbStatuses database.Statuses, port, internalPort string) (publicServer, internalServer *http.Server) {

	movieRepository := movies.NewMovieRepository(moviesDB, dbStatuses[database.MoviesDatabase])
	movieService := movies.NewMovieService(movieRepository)
	listRepository := recommendations.NewListRepository(recommendationsDB, dbStatuses[database.RecommendationsDatabase])
	cacheRepository := recommendations.NewRecommendationCacheRepository(recommendationsDB, dbStatuses[database.RecommendationsDatabase])
	embeddingRepository := recommendations.NewEmbeddingRepository(recommendationsDB, dbStatuses[database.RecommendationsDatabase])
	recommendationService := recommendations.NewService(movieService, embeddingRepository, listRepository, cacheRepository)
	var importedMovies atomic.Bool
	go func() {
		if err := movies.ImportPopularMoviesWithRetry(ctx, movieRepository, 10, 2*time.Second); err != nil {
			log.Printf("movie import failed: %v", err)
			return
		}
		importedMovies.Store(true)
		if err := recommendationService.InitializeWithRetry(ctx, 30, 2*time.Second); err != nil {
			log.Printf("recommendation models unavailable: %v", err)
		}
	}()
	userService := auth.NewUserService(auth.NewUserRepository(authDB, dbStatuses[database.AuthDatabase]), tokenIssuer)
	listService := recommendations.NewListService(listRepository, movieService)
	server := httpapi.NewServer(dbStatuses, &importedMovies, userService, movieService, listService, tokenVerifier, recommendationService)
	mux := http.NewServeMux()
	httpapi.RegisterEndpoints(mux, server)
	internalMux := http.NewServeMux()
	httpapi.RegisterInternalEndpoints(internalMux, server)

	publicServer = newHTTPServer(port, httpapi.RequestMetricsMiddleware(httpapi.CorsMiddleware(mux)))
	internalServer = newHTTPServer(internalPort, internalMux)
	return publicServer, internalServer
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
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

func portFromEnv(name, fallback string) string {
	port := os.Getenv(name)
	if port == "" {
		port = fallback
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		log.Printf("invalid %s %q", name, port)
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

	dbStatuses := database.Statuses{
		database.AuthDatabase:            database.NewDBStatus(authPool),
		database.MoviesDatabase:          database.NewDBStatus(moviesPool),
		database.RecommendationsDatabase: database.NewDBStatus(recommendationsPool),
	}
	for _, status := range dbStatuses {
		go status.Start(databaseContext, 5*time.Second)
	}

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

	port := portFromEnv("BACKEND_PORT", "8080")
	internalPort := portFromEnv("INTERNAL_PORT", "8081")
	if port == "" || internalPort == "" {
		return
	}

	publicServer, internalServer := setupServer(authPool, databaseContext, moviesPool, recommendationsPool, tokenIssuer, tokenVerifier, dbStatuses, port, internalPort)

	serverErrors := RunServers(publicServer, internalServer)

	HandleErrors(serverErrors, databaseContext, publicServer, internalServer)
}
