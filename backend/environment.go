package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func loadLocalEnvironment() error {
	for _, path := range []string{
		filepath.Join("..", "database", "auth.env"),
		filepath.Join("..", "database", "movies.env"),
	} {
		_, err := os.Stat(path)
		if err == nil {
			if err := godotenv.Load(path); err != nil {
				return fmt.Errorf("load environment file %q: %w", path, err)
			}
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("environment file %q not found", path)
		}
		return fmt.Errorf("check environment file %q: %w", path, err)
	}

	if err := godotenv.Load("tmdb.env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load environment file %q: %w", "tmdb.env", err)
	}

	return nil
}
