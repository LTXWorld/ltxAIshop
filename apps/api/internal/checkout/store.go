package checkout

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	OrderStatusPendingPayment = "pending_payment"
)

var (
	ErrProductUnavailable = errors.New("product unavailable")
	ErrCartIsEmpty        = errors.New("cart is empty")
	ErrOrderNotFound      = errors.New("order not found")
	ErrMixedCurrencies    = errors.New("cart contains mixed currencies")
)

type Cart struct {
	ID        int64
	UserID    int64
	Items     []CartItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CartItem struct {
	ID                  int64
	ProductID           int64
	ProductName         string
	ProductSlug         string
	PriceCents          int64
	Currency            string
	FulfillmentStrategy string
	Quantity            int
	LineTotalCents      int64
}

type Order struct {
	ID         int64
	UserID     int64
	TotalCents int64
	Currency   string
	Status     string
	Items      []OrderItem
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type OrderItem struct {
	ID                  int64
	ProductID           int64
	ProductName         string
	ProductSlug         string
	PriceCents          int64
	Currency            string
	FulfillmentStrategy string
	Quantity            int
	LineTotalCents      int64
}

type Store interface {
	GetCart(ctx context.Context, userID int64) (Cart, error)
	SetCartItem(ctx context.Context, userID int64, productID int64, quantity int) (Cart, error)
	RemoveCartItem(ctx context.Context, userID int64, productID int64) (Cart, error)
	CreateOrderFromCart(ctx context.Context, userID int64) (Order, error)
	ListOrders(ctx context.Context, userID int64) ([]Order, error)
	FindOrder(ctx context.Context, userID int64, orderID int64) (Order, error)
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) PostgresStore {
	return PostgresStore{db: db}
}

func (s PostgresStore) GetCart(ctx context.Context, userID int64) (Cart, error) {
	cartID, err := s.ensureCart(ctx, userID)
	if err != nil {
		return Cart{}, err
	}
	return s.loadCart(ctx, userID, cartID)
}

func (s PostgresStore) SetCartItem(ctx context.Context, userID int64, productID int64, quantity int) (Cart, error) {
	if quantity <= 0 {
		return s.RemoveCartItem(ctx, userID, productID)
	}

	var productPublished bool
	if err := s.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND status = 'published')", productID).Scan(&productPublished); err != nil {
		return Cart{}, err
	}
	if !productPublished {
		return Cart{}, ErrProductUnavailable
	}

	cartID, err := s.ensureCart(ctx, userID)
	if err != nil {
		return Cart{}, err
	}

	const query = `
INSERT INTO cart_items (cart_id, product_id, quantity)
VALUES ($1, $2, $3)
ON CONFLICT (cart_id, product_id) DO UPDATE
SET quantity = EXCLUDED.quantity,
    updated_at = now()`
	if _, err := s.db.Exec(ctx, query, cartID, productID, quantity); err != nil {
		return Cart{}, err
	}
	if _, err := s.db.Exec(ctx, "UPDATE carts SET updated_at = now() WHERE id = $1", cartID); err != nil {
		return Cart{}, err
	}
	return s.loadCart(ctx, userID, cartID)
}

func (s PostgresStore) RemoveCartItem(ctx context.Context, userID int64, productID int64) (Cart, error) {
	cartID, err := s.ensureCart(ctx, userID)
	if err != nil {
		return Cart{}, err
	}
	if _, err := s.db.Exec(ctx, "DELETE FROM cart_items WHERE cart_id = $1 AND product_id = $2", cartID, productID); err != nil {
		return Cart{}, err
	}
	if _, err := s.db.Exec(ctx, "UPDATE carts SET updated_at = now() WHERE id = $1", cartID); err != nil {
		return Cart{}, err
	}
	return s.loadCart(ctx, userID, cartID)
}

func (s PostgresStore) CreateOrderFromCart(ctx context.Context, userID int64) (Order, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cartID, err := ensureCartTx(ctx, tx, userID)
	if err != nil {
		return Order{}, err
	}

	const itemQuery = `
SELECT p.id, p.name, p.slug, p.price_cents, p.currency, p.fulfillment_strategy, ci.quantity
FROM cart_items ci
JOIN products p ON p.id = ci.product_id
WHERE ci.cart_id = $1 AND p.status = 'published'
ORDER BY ci.id
FOR UPDATE OF ci`
	rows, err := tx.Query(ctx, itemQuery, cartID)
	if err != nil {
		return Order{}, err
	}

	items := []OrderItem{}
	var total int64
	var currency string
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.ProductSlug, &item.PriceCents, &item.Currency, &item.FulfillmentStrategy, &item.Quantity); err != nil {
			rows.Close()
			return Order{}, err
		}
		if currency == "" {
			currency = item.Currency
		}
		if currency != item.Currency {
			rows.Close()
			return Order{}, ErrMixedCurrencies
		}
		item.LineTotalCents = item.PriceCents * int64(item.Quantity)
		total += item.LineTotalCents
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Order{}, err
	}
	rows.Close()

	if len(items) == 0 {
		return Order{}, ErrCartIsEmpty
	}

	var order Order
	const orderQuery = `
INSERT INTO orders (user_id, total_cents, currency, status)
VALUES ($1, $2, $3, 'pending_payment')
RETURNING id, user_id, total_cents, currency, status, created_at, updated_at`
	if err := tx.QueryRow(ctx, orderQuery, userID, total, currency).Scan(
		&order.ID,
		&order.UserID,
		&order.TotalCents,
		&order.Currency,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		return Order{}, err
	}

	for i := range items {
		const orderItemQuery = `
INSERT INTO order_items (order_id, product_id, product_name, product_slug, price_cents, currency, quantity, fulfillment_strategy)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id`
		if err := tx.QueryRow(
			ctx,
			orderItemQuery,
			order.ID,
			items[i].ProductID,
			items[i].ProductName,
			items[i].ProductSlug,
			items[i].PriceCents,
			items[i].Currency,
			items[i].Quantity,
			items[i].FulfillmentStrategy,
		).Scan(&items[i].ID); err != nil {
			return Order{}, err
		}
	}

	if _, err := tx.Exec(ctx, "DELETE FROM cart_items WHERE cart_id = $1", cartID); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE carts SET updated_at = now() WHERE id = $1", cartID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}

	order.Items = items
	return order, nil
}

func (s PostgresStore) ListOrders(ctx context.Context, userID int64) ([]Order, error) {
	const query = `
SELECT id, user_id, total_cents, currency, status, created_at, updated_at
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC, id DESC`
	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.TotalCents, &order.Currency, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range orders {
		items, err := s.loadOrderItems(ctx, orders[i].ID)
		if err != nil {
			return nil, err
		}
		orders[i].Items = items
	}
	return orders, nil
}

func (s PostgresStore) FindOrder(ctx context.Context, userID int64, orderID int64) (Order, error) {
	const query = `
SELECT id, user_id, total_cents, currency, status, created_at, updated_at
FROM orders
WHERE user_id = $1 AND id = $2`
	var order Order
	err := s.db.QueryRow(ctx, query, userID, orderID).Scan(
		&order.ID,
		&order.UserID,
		&order.TotalCents,
		&order.Currency,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}
	items, err := s.loadOrderItems(ctx, order.ID)
	if err != nil {
		return Order{}, err
	}
	order.Items = items
	return order, nil
}

func (s PostgresStore) ensureCart(ctx context.Context, userID int64) (int64, error) {
	return ensureCartTx(ctx, s.db, userID)
}

type cartExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ensureCartTx(ctx context.Context, q cartExecutor, userID int64) (int64, error) {
	const query = `
INSERT INTO carts (user_id)
VALUES ($1)
ON CONFLICT (user_id) DO UPDATE SET updated_at = carts.updated_at
RETURNING id`
	var cartID int64
	if err := q.QueryRow(ctx, query, userID).Scan(&cartID); err != nil {
		return 0, err
	}
	return cartID, nil
}

func (s PostgresStore) loadCart(ctx context.Context, userID int64, cartID int64) (Cart, error) {
	var cart Cart
	if err := s.db.QueryRow(ctx, "SELECT id, user_id, created_at, updated_at FROM carts WHERE id = $1 AND user_id = $2", cartID, userID).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.CreatedAt,
		&cart.UpdatedAt,
	); err != nil {
		return Cart{}, err
	}

	const itemQuery = `
SELECT ci.id, p.id, p.name, p.slug, p.price_cents, p.currency, p.fulfillment_strategy, ci.quantity
FROM cart_items ci
JOIN products p ON p.id = ci.product_id
WHERE ci.cart_id = $1
ORDER BY ci.id`
	rows, err := s.db.Query(ctx, itemQuery, cart.ID)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	cart.Items = []CartItem{}
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ID, &item.ProductID, &item.ProductName, &item.ProductSlug, &item.PriceCents, &item.Currency, &item.FulfillmentStrategy, &item.Quantity); err != nil {
			return Cart{}, err
		}
		item.LineTotalCents = item.PriceCents * int64(item.Quantity)
		cart.Items = append(cart.Items, item)
	}
	return cart, rows.Err()
}

func (s PostgresStore) loadOrderItems(ctx context.Context, orderID int64) ([]OrderItem, error) {
	const query = `
SELECT id, product_id, product_name, product_slug, price_cents, currency, fulfillment_strategy, quantity
FROM order_items
WHERE order_id = $1
ORDER BY id`
	rows, err := s.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []OrderItem{}
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ID, &item.ProductID, &item.ProductName, &item.ProductSlug, &item.PriceCents, &item.Currency, &item.FulfillmentStrategy, &item.Quantity); err != nil {
			return nil, err
		}
		item.LineTotalCents = item.PriceCents * int64(item.Quantity)
		items = append(items, item)
	}
	return items, rows.Err()
}
