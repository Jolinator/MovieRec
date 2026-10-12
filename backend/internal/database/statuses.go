package database

import (
	"context"
	"sync"
)

type Database uint8

const (
	AuthDatabase Database = iota
	MoviesDatabase
	RecommendationsDatabase
)

// Statuses groups the status of every database a process talks to.
type Statuses map[Database]*DBStatus

// Scan pings the given databases concurrently.
func (s Statuses) Scan(ctx context.Context, targets ...Database) {
	s.forEach(targets, func(status *DBStatus) { status.Scan(ctx) })
}

// Refresh rescans, concurrently, the databases currently believed to be unavailable.
func (s Statuses) Refresh(ctx context.Context) {
	s.forEach(s.databases(), func(status *DBStatus) { status.Refresh(ctx) })
}

func (s Statuses) databases() []Database {
	databases := make([]Database, 0, len(s))
	for database := range s {
		databases = append(databases, database)
	}
	return databases
}

func (s Statuses) forEach(targets []Database, action func(*DBStatus)) {
	var running sync.WaitGroup
	for _, target := range targets {
		status, ok := s[target]
		if !ok {
			continue
		}
		running.Add(1)
		go func() {
			defer running.Done()
			action(status)
		}()
	}
	running.Wait()
}
