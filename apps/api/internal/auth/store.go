package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RoleCustomer = "customer"
	RoleAdmin    = "admin"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUserNotFound           = errors.New("user not found")
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Store interface {
	CreateUser(ctx context.Context, email string, passwordHash string, role string) (User, error)
	FindUserByEmail(ctx context.Context, email string) (User, error)
	FindUserByID(ctx context.Context, id int64) (User, error)
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) PostgresStore {
	return PostgresStore{db: db}
}

func (s PostgresStore) CreateUser(ctx context.Context, email string, passwordHash string, role string) (User, error) {
	const query = `
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, $3)
RETURNING id, email, password_hash, role, created_at, updated_at`

	var user User
	err := s.db.QueryRow(ctx, query, normalizeEmail(email), passwordHash, role).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailAlreadyRegistered
		}
		return User{}, err
	}

	return user, nil
}

func (s PostgresStore) FindUserByEmail(ctx context.Context, email string) (User, error) {
	const query = `
SELECT id, email, password_hash, role, created_at, updated_at
FROM users
WHERE email = $1`
	return s.findOne(ctx, query, normalizeEmail(email))
}

func (s PostgresStore) FindUserByID(ctx context.Context, id int64) (User, error) {
	const query = `
SELECT id, email, password_hash, role, created_at, updated_at
FROM users
WHERE id = $1`
	return s.findOne(ctx, query, id)
}

func (s PostgresStore) findOne(ctx context.Context, query string, arg any) (User, error) {
	var user User
	err := s.db.QueryRow(ctx, query, arg).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
