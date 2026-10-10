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

	mu                       sync.RWMutex
	authAvailable            bool
	moviesAvailable          bool
	recommendationsAvailable bool
}

func NewDBStatus(authDB, moviesDB, recommendationsDB *pgxpool.Pool) *DBStatus {
	return &DBStatus{authDB: authDB, moviesDB: moviesDB, recommendationsDB: recommendationsDB}
}

func (s *DBStatus) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	s.check(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.check(ctx)
		}
	}
}

func (s *DBStatus) check(ctx context.Context) {
	var authErr, moviesErr, recommendationsErr error
	var checks sync.WaitGroup
	checks.Add(3)
	go func() {
		defer checks.Done()
		authErr = PingWithRetry(ctx, s.authDB, 2, 500*time.Millisecond)
	}()
	go func() {
		defer checks.Done()
		moviesErr = PingWithRetry(ctx, s.moviesDB, 2, 500*time.Millisecond)
	}()
	go func() {
		defer checks.Done()
		recommendationsErr = PingWithRetry(ctx, s.recommendationsDB, 2, 500*time.Millisecond)
	}()
	checks.Wait()

	s.mu.Lock()
	s.authAvailable = authErr == nil
	s.moviesAvailable = moviesErr == nil
	s.recommendationsAvailable = recommendationsErr == nil
	s.mu.Unlock()
}

func (s *DBStatus) RecommendationsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recommendationsAvailable
}

func (s *DBStatus) AuthAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authAvailable
}

func (s *DBStatus) MoviesAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.moviesAvailable
}
