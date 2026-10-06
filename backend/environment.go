package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func loadLocalEnvironment() error {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"),
	}

	for _, path := range candidates {
		_, err := os.Stat(path)
		if err == nil {
			if err := godotenv.Load(path); err != nil {
				return fmt.Errorf("load environment file %q: %w", path, err)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check environment file %q: %w", path, err)
		}
	}

	return nil
}
