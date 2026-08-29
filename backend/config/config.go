package config

import (
	"os"
	"path/filepath"
)

// projectRoot returns the repo root when the server is started from backend/.
func projectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if filepath.Base(cwd) == "backend" {
		return filepath.Dir(cwd)
	}
	return cwd
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
