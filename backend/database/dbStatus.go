package database

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database uint8

const (
	AuthDatabase Database = iota
	MoviesDatabase
	RecommendationsDatabase
)

type DBStatus struct {
	authDB            *pgxpool.Pool
	moviesDB          *pgxpool.Pool
	recommendationsDB *pgxpool.Pool

	scanMu                     sync.Mutex
	mu                         sync.RWMutex
	authDbAvailable            bool
	moviesDbAvailable          bool
	importedMoviesAvailable    bool
	recommendationsDbAvailable bool
}

func NewDBStatus(authDB, moviesDB, recommendationsDB *pgxpool.Pool) *DBStatus {
	return &DBStatus{authDB: authDB, moviesDB: moviesDB, recommendationsDB: recommendationsDB}
}

func (s *DBStatus) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	s.Scan(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Scan(ctx)
		}
	}
}

func (s *DBStatus) HealthScan(ctx context.Context) {
	var unavailable []Database
	s.mu.RLock()
	if !s.authDbAvailable {
		unavailable = append(unavailable, AuthDatabase)
	}
	if !s.moviesDbAvailable {
		unavailable = append(unavailable, MoviesDatabase)
	}
	if !s.recommendationsDbAvailable {
		unavailable = append(unavailable, RecommendationsDatabase)
	}
	s.mu.RUnlock()
	if len(unavailable) > 0 {
		s.Scan(ctx, unavailable...)
	}
}

func (s *DBStatus) Scan(ctx context.Context, databases ...Database) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if ctx.Err() != nil {
		return
	}

	if len(databases) == 0 {
		databases = []Database{AuthDatabase, MoviesDatabase, RecommendationsDatabase}
	}
	checksAuth, checksMovies, checksRecommendations := false, false, false
	for _, database := range databases {
		switch database {
		case AuthDatabase:
			checksAuth = true
		case MoviesDatabase:
			checksMovies = true
		case RecommendationsDatabase:
			checksRecommendations = true
		default:
			return
		}
	}

	var authErr, moviesErr, recommendationsErr error
	var checks sync.WaitGroup
	if checksAuth {
		checks.Add(1)
		go func() {
			defer checks.Done()
			authErr = s.ping(ctx, s.authDB)
		}()
	}
	if checksMovies {
		checks.Add(1)
		go func() {
			defer checks.Done()
			moviesErr = s.ping(ctx, s.moviesDB)
		}()
	}
	if checksRecommendations {
		checks.Add(1)
		go func() {
			defer checks.Done()
			recommendationsErr = s.ping(ctx, s.recommendationsDB)
		}()
	}
	checks.Wait()
	if ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	if checksAuth {
		s.authDbAvailable = authErr == nil
	}
	if checksMovies {
		s.moviesDbAvailable = moviesErr == nil
	}
	if checksRecommendations {
		s.recommendationsDbAvailable = recommendationsErr == nil
	}
	s.mu.Unlock()
}

func (s *DBStatus) ping(ctx context.Context, pool *pgxpool.Pool) error {
	pingContext, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return pool.Ping(pingContext)
}

func (s *DBStatus) SetImportedMoviesAvailable(available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.importedMoviesAvailable = available
}

func (s *DBStatus) RecommendationsDbAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recommendationsDbAvailable
}

func (s *DBStatus) AuthAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authDbAvailable
}

func (s *DBStatus) MoviesAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.moviesDbAvailable
}

func (s *DBStatus) ImportedMoviesAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.importedMoviesAvailable
}
