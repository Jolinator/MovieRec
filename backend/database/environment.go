package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func LoadLocalEnvironment() error {
	for _, name := range []string{"auth.env", "movies.env", "recommendations.env", "tmdb.env", "ports.env"} {
		path := filepath.Join("..", "secrets", name)
		_, err := os.Stat(path)
		if err == nil {
			if err := godotenv.Load(path); err != nil {
				return fmt.Errorf("load environment file %q: %w", path, err)
			}
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			// In container platforms, configuration is supplied through the
			// process environment (for example, Kubernetes Secrets). Local
			// Compose development can continue to load these files when present.
			continue
		}
		return fmt.Errorf("check environment file %q: %w", path, err)
	}

	return nil
}
