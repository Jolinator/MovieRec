package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type databaseConfig struct {
	host     string
	port     uint16
	name     string
	user     string
	password string
	sslMode  string
}

func loadDatabaseConfig() (databaseConfig, error) {
	portValue := os.Getenv("DB_PORT")
	if portValue == "" {
		portValue = "5432"
	}

	port, err := strconv.ParseUint(portValue, 10, 16)
	if err != nil || port == 0 {
		return databaseConfig{}, fmt.Errorf("invalid DB_PORT %q", portValue)
	}

	config := databaseConfig{
		host:     os.Getenv("DB_HOST"),
		port:     uint16(port),
		name:     os.Getenv("DB_NAME"),
		user:     os.Getenv("DB_USER"),
		password: os.Getenv("DB_PASSWORD"),
		sslMode:  os.Getenv("DB_SSLMODE"),
	}

	if config.host == "" {
		config.host = "localhost"
	}
	if config.name == "" {
		config.name = "auth"
	}
	if config.user == "" {
		config.user = "moviehub"
	}
	if config.password == "" {
		return databaseConfig{}, fmt.Errorf("DB_PASSWORD must be set")
	}
	if config.sslMode == "" {
		config.sslMode = "disable"
	}

	return config, nil
}

func openDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	config, err := loadDatabaseConfig()
	if err != nil {
		return nil, err
	}

	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.user, config.password),
		Host:   net.JoinHostPort(config.host, strconv.FormatUint(uint64(config.port), 10)),
		Path:   "/" + config.name,
	}
	query := connectionURL.Query()
	query.Set("sslmode", config.sslMode)
	connectionURL.RawQuery = query.Encode()

	poolConfig, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return pool, nil
}

func openDatabaseWithRetry(ctx context.Context, attempts int, delay time.Duration) (*pgxpool.Pool, error) {
	var lastError error

	for attempt := 1; attempt <= attempts; attempt++ {
		pool, err := openDatabase(ctx)
		if err == nil {
			return pool, nil
		}
		lastError = err

		if attempt == attempts {
			break
		}

		log.Printf("database connection attempt %d/%d failed: %v; retrying in %s", attempt, attempts, err, delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf("database connection failed after %d attempts: %w", attempts, lastError)
}
