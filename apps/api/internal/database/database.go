package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := Ping(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("database pool is required")
	}
	return pool.Ping(ctx)
}

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, path string) error {
	if pool == nil {
		return errors.New("database pool is required")
	}
	if path == "" {
		return errors.New("migrations path is required")
	}

	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT FALSE
)`); err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(path, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, file := range files {
		version, err := migrationVersion(file)
		if err != nil {
			return err
		}

		var exists bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		sql, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)", version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	return nil
}

func migrationVersion(file string) (int64, error) {
	base := filepath.Base(file)
	prefix, _, ok := strings.Cut(base, "_")
	if !ok {
		return 0, errors.New("migration filename must start with a version")
	}
	return strconv.ParseInt(prefix, 10, 64)
}
