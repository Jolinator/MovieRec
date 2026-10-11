package database

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBStatus struct {
	authDB            *pgxpool.Pool
	moviesDB          *pgxpool.Pool
	recommendationsDB *pgxpool.Pool

	scanMu                   sync.Mutex
	mu                       sync.RWMutex
	authDbAvailable          bool
	moviesDbAvailable        bool
	importedMoviesAvailable  bool
	recommendationsAvailable bool
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

func (s *DBStatus) Scan(ctx context.Context) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if ctx.Err() != nil {
		return
	}

	var authErr, moviesErr, recommendationsErr error
	var checks sync.WaitGroup
	checks.Add(3)
	go func() {
		defer checks.Done()
		authErr = s.ping(ctx, s.authDB)
	}()
	go func() {
		defer checks.Done()
		moviesErr = s.ping(ctx, s.moviesDB)
	}()
	go func() {
		defer checks.Done()
		recommendationsErr = s.ping(ctx, s.recommendationsDB)
	}()
	checks.Wait()
	if ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	s.authDbAvailable = authErr == nil
	s.moviesDbAvailable = moviesErr == nil
	s.recommendationsAvailable = recommendationsErr == nil
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

func (s *DBStatus) RecommendationsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recommendationsAvailable
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
