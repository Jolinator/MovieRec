package database

import (
	"context"
	"sync/atomic"
	"time"

	"backend/internal/observability"
)

const (
	defaultScanInterval = 5 * time.Second
	pingTimeout         = time.Second
)

type pinger interface {
	Ping(ctx context.Context) error
}

// DBStatus tracks the availability of a single database.
type DBStatus struct {
	db        pinger
	name      string
	available atomic.Bool
}

func NewDBStatus(db pinger, name ...string) *DBStatus {
	databaseName := "unknown"
	if len(name) > 0 && name[0] != "" {
		databaseName = name[0]
	}
	status := &DBStatus{db: db, name: databaseName}
	observability.ObserveDatabaseAvailability(databaseName, false)
	return status
}

func (s *DBStatus) Available() bool {
	return s.available.Load()
}

// Start scans the database periodically until ctx is cancelled.
func (s *DBStatus) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = defaultScanInterval
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

// Scan pings the database and records the result. A cancelled ctx keeps the last known state.
func (s *DBStatus) Scan(ctx context.Context) {
	err := s.ping(ctx)
	if ctx.Err() != nil {
		return
	}
	s.available.Store(err == nil)
	observability.ObserveDatabaseAvailability(s.name, err == nil)
}

// Refresh rescans only when the database is currently believed to be unavailable.
func (s *DBStatus) Refresh(ctx context.Context) {
	if !s.Available() {
		s.Scan(ctx)
	}
}

func (s *DBStatus) ping(ctx context.Context) error {
	pingContext, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return s.db.Ping(pingContext)
}
