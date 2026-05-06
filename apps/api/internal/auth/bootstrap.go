package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureBootstrapAdmin(ctx context.Context, db *pgxpool.Pool, email string, password string) error {
	if email == "" && password == "" {
		return nil
	}
	if email == "" || password == "" {
		return errors.New("admin email and password must be set together")
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, 'admin')
ON CONFLICT (email) DO UPDATE
SET role = 'admin',
    updated_at = now()
WHERE users.role <> 'admin'`
	_, err = db.Exec(ctx, query, normalizeEmail(email), passwordHash)
	return err
}
