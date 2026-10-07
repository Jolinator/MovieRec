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

func loadDatabaseConfig(prefix string) (databaseConfig, error) {
	portValue := os.Getenv(prefix + "DB_PORT")
	if portValue == "" {
		portValue = "5432"
	}

	port, err := strconv.ParseUint(portValue, 10, 16)
	if err != nil || port == 0 {
		return databaseConfig{}, fmt.Errorf("invalid DB_PORT %q", portValue)
	}

	config := databaseConfig{
		host:     os.Getenv(prefix + "DB_HOST"),
		port:     uint16(port),
<<<<<<< HEAD:backend/connectDB.go
		name:     os.Getenv("POSTGRES_DB"),
		user:     os.Getenv("POSTGRES_USER"),
		password: os.Getenv("POSTGRES_PASSWORD"),
		sslMode:  os.Getenv("DB_SSLMODE"),
=======
		name:     os.Getenv(prefix + "DB_NAME"),
		user:     os.Getenv(prefix + "DB_USER"),
		password: os.Getenv(prefix + "DB_PASSWORD"),
		sslMode:  os.Getenv(prefix + "DB_SSLMODE"),
	}
	if prefix == "" {
		config.name = os.Getenv("POSTGRES_DB")
		config.user = os.Getenv("POSTGRES_USER")
		config.password = os.Getenv("POSTGRES_PASSWORD")
>>>>>>> c27a1b0 (new movie database):backend/database.go
	}

	if config.host == "" {
		return databaseConfig{}, fmt.Errorf("DB_HOST must be set")
	}
	if config.name == "" {
		return databaseConfig{}, fmt.Errorf("%sDB_NAME must be set", prefix)
	}
	if config.user == "" {
		return databaseConfig{}, fmt.Errorf("%sDB_USER must be set", prefix)
	}
	if config.password == "" {
<<<<<<< HEAD:backend/connectDB.go
		return databaseConfig{}, fmt.Errorf("POSTGRES_PASSWORD must be set")
=======
		return databaseConfig{}, fmt.Errorf("%sDB_PASSWORD must be set", prefix)
>>>>>>> c27a1b0 (new movie database):backend/database.go
	}
	if config.sslMode == "" {
		return databaseConfig{}, fmt.Errorf("DB_SSLMODE must be set")
	}

	return config, nil
}

<<<<<<< HEAD:backend/connectDB.go
func setupConnectionURL(config databaseConfig) url.URL {
	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.user, config.password),
		Host:   net.JoinHostPort(config.host, strconv.FormatUint(uint64(config.port), 10)),
		Path:   "/" + config.name,
	}
	query := connectionURL.Query()
	query.Set("sslmode", config.sslMode)
	connectionURL.RawQuery = query.Encode()
	return connectionURL
}

func setupPool(ctx context.Context) (*pgxpool.Pool, error) {

	config, err := loadDatabaseConfig()
=======

func setupPool(ctx context.Context, prefix string) (*pgxpool.Pool, error) {

	config, err := loadDatabaseConfig(prefix)
>>>>>>> c27a1b0 (new movie database):backend/database.go
	if err != nil {
		return nil, err
	}

	connectionURL := setupConnectionURL(config)

	poolConfig, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return pool, nil
}

func attemptPoolPing(ctx context.Context, pool *pgxpool.Pool) error {
	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return fmt.Errorf("connect to database: %w", err)
	}
	return nil
}

<<<<<<< HEAD:backend/connectDB.go
func openDatabase(ctx context.Context) (*pgxpool.Pool, error) {

	pool, err := setupPool(ctx)
=======
func openDatabase(ctx context.Context, prefix string) (*pgxpool.Pool, error) {

	pool, err := setupPool(ctx, prefix)
>>>>>>> c27a1b0 (new movie database):backend/database.go
	if err != nil {
		return nil, err
	}

	err = attemptPoolPing(ctx, pool)
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func openDatabaseWithRetry(ctx context.Context, prefix string, attempts int, delay time.Duration) (*pgxpool.Pool, error) {
	var lastError error

	for attempt := 1; attempt <= attempts; attempt++ {
		pool, err := openDatabase(ctx, prefix)
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

	return nil, fmt.Errorf("%sdatabase connection failed after %d attempts: %w", prefix, attempts, lastError)
}
