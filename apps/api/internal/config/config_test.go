package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://ltxai:ltxai@localhost:5432/ltxai_shop?sslmode=disable")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("WEB_ORIGIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL should be set")
	}
	if cfg.MigrationsPath != "migrations" {
		t.Fatalf("MigrationsPath = %q, want migrations", cfg.MigrationsPath)
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("PublicBaseURL = %q, want http://localhost:8080", cfg.PublicBaseURL)
	}
	if cfg.WebOrigin != "http://localhost:5173" {
		t.Fatalf("WebOrigin = %q, want http://localhost:5173", cfg.WebOrigin)
	}
	if cfg.AuthTokenKey == "" {
		t.Fatal("AuthTokenKey should be set")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want missing DATABASE_URL error")
	}
}

func TestLoadRequiresTokenKeyInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://ltxai:ltxai@localhost:5432/ltxai_shop?sslmode=disable")
	t.Setenv("AUTH_TOKEN_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want missing AUTH_TOKEN_KEY error")
	}
}

func TestLoadRequiresCompleteAdminBootstrapConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://ltxai:ltxai@localhost:5432/ltxai_shop?sslmode=disable")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want incomplete admin bootstrap error")
	}
}
