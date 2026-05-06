package catalog

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
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"

	FulfillmentDigitalCredentials = "digital_credentials"
	FulfillmentManualContact      = "manual_contact"
	FulfillmentDigitalCode        = "digital_code"
)

var (
	ErrProductNotFound      = errors.New("product not found")
	ErrProductSlugExists    = errors.New("product slug already exists")
	ErrInvalidProductStatus = errors.New("invalid product status")
)

type Product struct {
	ID                  int64
	Name                string
	Slug                string
	Description         string
	PriceCents          int64
	Currency            string
	Status              string
	FulfillmentStrategy string
	ImageURL            string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateProductParams struct {
	Name                string
	Slug                string
	Description         string
	PriceCents          int64
	Currency            string
	Status              string
	FulfillmentStrategy string
	ImageURL            string
}

type UpdateProductParams struct {
	ID                  int64
	Name                string
	Slug                string
	Description         string
	PriceCents          int64
	Currency            string
	Status              string
	FulfillmentStrategy string
	ImageURL            string
}

type Store interface {
	ListPublishedProducts(ctx context.Context) ([]Product, error)
	FindPublishedProductBySlug(ctx context.Context, slug string) (Product, error)
	ListAdminProducts(ctx context.Context) ([]Product, error)
	CreateProduct(ctx context.Context, params CreateProductParams) (Product, error)
	UpdateProduct(ctx context.Context, params UpdateProductParams) (Product, error)
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) PostgresStore {
	return PostgresStore{db: db}
}

func (s PostgresStore) ListPublishedProducts(ctx context.Context) ([]Product, error) {
	const query = `
SELECT id, name, slug, description, price_cents, currency, status, fulfillment_strategy, image_url, created_at, updated_at
FROM products
WHERE status = 'published'
ORDER BY created_at DESC, id DESC`
	return s.list(ctx, query)
}

func (s PostgresStore) FindPublishedProductBySlug(ctx context.Context, slug string) (Product, error) {
	const query = `
SELECT id, name, slug, description, price_cents, currency, status, fulfillment_strategy, image_url, created_at, updated_at
FROM products
WHERE status = 'published' AND slug = $1`
	return s.findOne(ctx, query, normalizeSlug(slug))
}

func (s PostgresStore) ListAdminProducts(ctx context.Context) ([]Product, error) {
	const query = `
SELECT id, name, slug, description, price_cents, currency, status, fulfillment_strategy, image_url, created_at, updated_at
FROM products
ORDER BY created_at DESC, id DESC`
	return s.list(ctx, query)
}

func (s PostgresStore) CreateProduct(ctx context.Context, params CreateProductParams) (Product, error) {
	const query = `
INSERT INTO products (name, slug, description, price_cents, currency, status, fulfillment_strategy, image_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, name, slug, description, price_cents, currency, status, fulfillment_strategy, image_url, created_at, updated_at`

	return s.findOne(
		ctx,
		query,
		strings.TrimSpace(params.Name),
		normalizeSlug(params.Slug),
		strings.TrimSpace(params.Description),
		params.PriceCents,
		normalizeCurrency(params.Currency),
		params.Status,
		params.FulfillmentStrategy,
		strings.TrimSpace(params.ImageURL),
	)
}

func (s PostgresStore) UpdateProduct(ctx context.Context, params UpdateProductParams) (Product, error) {
	const query = `
UPDATE products
SET name = $2,
    slug = $3,
    description = $4,
    price_cents = $5,
    currency = $6,
    status = $7,
    fulfillment_strategy = $8,
    image_url = $9,
    updated_at = now()
WHERE id = $1
RETURNING id, name, slug, description, price_cents, currency, status, fulfillment_strategy, image_url, created_at, updated_at`

	return s.findOne(
		ctx,
		query,
		params.ID,
		strings.TrimSpace(params.Name),
		normalizeSlug(params.Slug),
		strings.TrimSpace(params.Description),
		params.PriceCents,
		normalizeCurrency(params.Currency),
		params.Status,
		params.FulfillmentStrategy,
		strings.TrimSpace(params.ImageURL),
	)
}

func (s PostgresStore) list(ctx context.Context, query string) ([]Product, error) {
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (s PostgresStore) findOne(ctx context.Context, query string, args ...any) (Product, error) {
	product, err := scanProduct(s.db.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrProductNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Product{}, ErrProductSlugExists
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return Product{}, ErrInvalidProductStatus
		}
		return Product{}, err
	}
	return product, nil
}

type productScanner interface {
	Scan(dest ...any) error
}

func scanProduct(row productScanner) (Product, error) {
	var product Product
	err := row.Scan(
		&product.ID,
		&product.Name,
		&product.Slug,
		&product.Description,
		&product.PriceCents,
		&product.Currency,
		&product.Status,
		&product.FulfillmentStrategy,
		&product.ImageURL,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	return product, err
}

func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func normalizeCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return "CNY"
	}
	return currency
}
