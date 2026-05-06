package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ltxai/shop/apps/api/internal/auth"
	"github.com/ltxai/shop/apps/api/internal/catalog"
	"github.com/ltxai/shop/apps/api/internal/config"
	"github.com/ltxai/shop/apps/api/internal/database"
	"github.com/ltxai/shop/apps/api/internal/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.ApplyMigrations(ctx, db, cfg.MigrationsPath); err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	if err := auth.EnsureBootstrapAdmin(ctx, db, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		logger.Error("bootstrap admin", "error", err)
		os.Exit(1)
	}

	authHandler := auth.NewHandler(auth.NewPostgresStore(db), auth.NewTokenManager(cfg.AuthTokenKey))
	catalogHandler := catalog.NewHandler(catalog.NewPostgresStore(db))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.NewRouter(httpserver.WithAuth(authHandler), httpserver.WithCatalog(catalogHandler)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
		os.Exit(1)
	}
}
