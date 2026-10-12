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
	"backend/internal/database"
	"backend/internal/gateway"
	"backend/internal/movies"
	"backend/internal/platform"
	"backend/internal/recommendations"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// setupServer wires every service into one process; each service registers its own routes.
func setupServer(authDB *pgxpool.Pool, ctx context.Context, moviesDB, recommendationsDB *pgxpool.Pool, tokenIssuer *auth.TokenIssuer, tokenVerifier *platform.TokenVerifier, dbStatuses database.Statuses, port, internalPort string) (publicServer, internalServer *http.Server) {

	movieRepository := movies.NewMovieRepository(moviesDB, dbStatuses[database.MoviesDatabase])
	movieService := movies.NewMovieService(movieRepository)
	moviesClient := recommendations.NewMoviesClient()
	listRepository := recommendations.NewListRepository(recommendationsDB, dbStatuses[database.RecommendationsDatabase])
	cacheRepository := recommendations.NewRecommendationCacheRepository(recommendationsDB, dbStatuses[database.RecommendationsDatabase])
	embeddingRepository := recommendations.NewEmbeddingRepository(recommendationsDB, dbStatuses[database.RecommendationsDatabase])
	recommendationService := recommendations.NewService(moviesClient, embeddingRepository, listRepository, cacheRepository)
	go func() {
		if err := movies.ImportPopularMoviesWithRetry(ctx, movieRepository, 10, 2*time.Second); err != nil {
			log.Printf("movie import failed: %v", err)
			return
		}
		movieService.MarkImported()
		if err := recommendationService.InitializeWithRetry(ctx, 30, 2*time.Second); err != nil {
			log.Printf("recommendation models unavailable: %v", err)
		}
	}()
	userService := auth.NewUserService(auth.NewUserRepository(authDB, dbStatuses[database.AuthDatabase]), tokenIssuer)
	listService := recommendations.NewListService(listRepository, moviesClient)

	authenticated := platform.AuthenticationMiddleware(tokenVerifier)
	moviesHandler := movies.NewHandler(movieService)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	health := platform.HealthHandler(platform.HealthSources{
		Statuses:             dbStatuses,
		MoviesImported:       movieService.Imported,
		RecommendationsReady: recommendationService.Ready,
	})
	mux.HandleFunc("GET /movieAPI/{$}", health)
	mux.HandleFunc("GET /movieAPI/health", health)
	moviesHandler.RegisterRoutes(mux)
	auth.NewHandler(userService, tokenVerifier, recommendationService).RegisterRoutes(mux, authenticated)
	recommendations.NewHandler(recommendationService, listService).RegisterRoutes(mux, authenticated)
	mux.HandleFunc("/", gateway.RedirectToMovieAPI)
	internalMux := http.NewServeMux()
	moviesHandler.RegisterInternalRoutes(internalMux)

	publicServer = platform.NewHTTPServer(port, platform.RequestMetricsMiddleware(gateway.CorsMiddleware(mux)))
	internalServer = platform.NewHTTPServer(internalPort, internalMux)
	return publicServer, internalServer
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
	tokenVerifier, err := platform.SetupTokenVerifier()
	if err != nil {
		log.Printf("JWT verifier startup failed: %v", err)
		return
	}

	port := platform.PortFromEnv("BACKEND_PORT", "8080")
	internalPort := platform.PortFromEnv("INTERNAL_PORT", "8081")
	if port == "" || internalPort == "" {
		return
	}

	publicServer, internalServer := setupServer(authPool, databaseContext, moviesPool, recommendationsPool, tokenIssuer, tokenVerifier, dbStatuses, port, internalPort)

	serverErrors := platform.RunServers(publicServer, internalServer)

	platform.HandleErrors(serverErrors, databaseContext, publicServer, internalServer)
}
