package config

import (
	"errors"
	"os"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	DatabaseURL    string
	MigrationsPath string
	PublicBaseURL  string
	WebOrigin      string
	AuthTokenKey   string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "migrations"),
		PublicBaseURL:  getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		WebOrigin:      getEnv("WEB_ORIGIN", "http://localhost:5173"),
		AuthTokenKey:   getEnv("AUTH_TOKEN_KEY", "development-insecure-change-me"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.AppEnv == "production" && cfg.AuthTokenKey == "development-insecure-change-me" {
		return Config{}, errors.New("AUTH_TOKEN_KEY is required in production")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
