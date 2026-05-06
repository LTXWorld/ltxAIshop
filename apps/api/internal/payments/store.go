package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ProviderManual = "manual"
	ProviderAlipay = "alipay"

	StatusCreated   = "created"
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"

	OrderStatusPendingPayment = "pending_payment"
	OrderStatusPaid           = "paid"
)

var (
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderNotPayable       = errors.New("order is not payable")
	ErrPaymentNotFound       = errors.New("payment not found")
	ErrPaymentAmountMismatch = errors.New("payment amount mismatch")
)

type Payment struct {
	ID              int64
	OrderID         int64
	UserID          int64
	Provider        string
	MerchantOrderNo string
	ProviderTradeNo string
	AmountCents     int64
	Currency        string
	Status          string
	RawPayload      map[string]any
	PaidAt          *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreatePaymentParams struct {
	UserID   int64
	OrderID  int64
	Provider string
}

type ConfirmPaymentParams struct {
	UserID          int64
	PaymentID       int64
	AmountCents     int64
	ProviderTradeNo string
	RawPayload      map[string]any
}

type Store interface {
	CreatePayment(ctx context.Context, params CreatePaymentParams) (Payment, error)
	ConfirmPayment(ctx context.Context, params ConfirmPaymentParams) (Payment, error)
	ConfirmProviderPayment(ctx context.Context, merchantOrderNo string, amountCents int64, providerTradeNo string, rawPayload map[string]any) (Payment, error)
	FindPayment(ctx context.Context, userID int64, paymentID int64) (Payment, error)
}

type PostgresStore struct {
	db  *pgxpool.Pool
	now func() time.Time
}

func NewPostgresStore(db *pgxpool.Pool) PostgresStore {
	return PostgresStore{db: db, now: time.Now}
}

func (s PostgresStore) CreatePayment(ctx context.Context, params CreatePaymentParams) (Payment, error) {
	provider := params.Provider
	if provider == "" {
		provider = ProviderManual
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Payment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderAmount int64
	var orderCurrency string
	var orderStatus string
	const orderQuery = `
SELECT total_cents, currency, status
FROM orders
WHERE id = $1 AND user_id = $2
FOR UPDATE`
	err = tx.QueryRow(ctx, orderQuery, params.OrderID, params.UserID).Scan(&orderAmount, &orderCurrency, &orderStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, ErrOrderNotFound
	}
	if err != nil {
		return Payment{}, err
	}
	if orderStatus != OrderStatusPendingPayment {
		return Payment{}, ErrOrderNotPayable
	}

	const existingQuery = `
SELECT id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at
FROM payments
WHERE order_id = $1 AND user_id = $2 AND status IN ('created', 'pending', 'succeeded')
ORDER BY id DESC
LIMIT 1`
	payment, err := scanPayment(tx.QueryRow(ctx, existingQuery, params.OrderID, params.UserID))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return Payment{}, err
		}
		return payment, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, err
	}

	merchantOrderNo := fmt.Sprintf("LTX%d%06d", s.now().Unix(), params.OrderID)
	const insertQuery = `
INSERT INTO payments (order_id, user_id, provider, merchant_order_no, amount_cents, currency, status)
VALUES ($1, $2, $3, $4, $5, $6, 'created')
RETURNING id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at`
	payment, err = scanPayment(tx.QueryRow(ctx, insertQuery, params.OrderID, params.UserID, provider, merchantOrderNo, orderAmount, orderCurrency))
	if err != nil {
		return Payment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Payment{}, err
	}
	return payment, nil
}

func (s PostgresStore) ConfirmPayment(ctx context.Context, params ConfirmPaymentParams) (Payment, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Payment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const paymentQuery = `
SELECT id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at
FROM payments
WHERE id = $1 AND user_id = $2
FOR UPDATE`
	payment, err := scanPayment(tx.QueryRow(ctx, paymentQuery, params.PaymentID, params.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, ErrPaymentNotFound
	}
	if err != nil {
		return Payment{}, err
	}
	if payment.AmountCents != params.AmountCents {
		return Payment{}, ErrPaymentAmountMismatch
	}
	if payment.Status == StatusSucceeded {
		if err := tx.Commit(ctx); err != nil {
			return Payment{}, err
		}
		return payment, nil
	}

	rawPayload, err := json.Marshal(params.RawPayload)
	if err != nil {
		return Payment{}, err
	}
	paidAt := s.now()
	const updatePaymentQuery = `
UPDATE payments
SET status = 'succeeded',
    provider_trade_no = $3,
    raw_payload = $4,
    paid_at = $5,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at`
	payment, err = scanPayment(tx.QueryRow(ctx, updatePaymentQuery, params.PaymentID, params.UserID, params.ProviderTradeNo, rawPayload, paidAt))
	if err != nil {
		return Payment{}, err
	}

	const updateOrderQuery = `
UPDATE orders
SET status = 'paid',
    updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'pending_payment'`
	if _, err := tx.Exec(ctx, updateOrderQuery, payment.OrderID, payment.UserID); err != nil {
		return Payment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Payment{}, err
	}
	return payment, nil
}

func (s PostgresStore) ConfirmProviderPayment(ctx context.Context, merchantOrderNo string, amountCents int64, providerTradeNo string, rawPayload map[string]any) (Payment, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Payment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const paymentQuery = `
SELECT id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at
FROM payments
WHERE merchant_order_no = $1
FOR UPDATE`
	payment, err := scanPayment(tx.QueryRow(ctx, paymentQuery, merchantOrderNo))
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, ErrPaymentNotFound
	}
	if err != nil {
		return Payment{}, err
	}
	if payment.AmountCents != amountCents {
		return Payment{}, ErrPaymentAmountMismatch
	}
	if payment.Status == StatusSucceeded {
		if err := tx.Commit(ctx); err != nil {
			return Payment{}, err
		}
		return payment, nil
	}

	rawPayloadJSON, err := json.Marshal(rawPayload)
	if err != nil {
		return Payment{}, err
	}
	paidAt := s.now()
	const updatePaymentQuery = `
UPDATE payments
SET status = 'succeeded',
    provider_trade_no = $2,
    raw_payload = $3,
    paid_at = $4,
    updated_at = now()
WHERE id = $1
RETURNING id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at`
	payment, err = scanPayment(tx.QueryRow(ctx, updatePaymentQuery, payment.ID, providerTradeNo, rawPayloadJSON, paidAt))
	if err != nil {
		return Payment{}, err
	}

	const updateOrderQuery = `
UPDATE orders
SET status = 'paid',
    updated_at = now()
WHERE id = $1 AND status = 'pending_payment'`
	if _, err := tx.Exec(ctx, updateOrderQuery, payment.OrderID); err != nil {
		return Payment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Payment{}, err
	}
	return payment, nil
}

func (s PostgresStore) FindPayment(ctx context.Context, userID int64, paymentID int64) (Payment, error) {
	const query = `
SELECT id, order_id, user_id, provider, merchant_order_no, provider_trade_no, amount_cents, currency, status, raw_payload, paid_at, created_at, updated_at
FROM payments
WHERE id = $1 AND user_id = $2`
	payment, err := scanPayment(s.db.QueryRow(ctx, query, paymentID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, ErrPaymentNotFound
	}
	return payment, err
}

type paymentScanner interface {
	Scan(dest ...any) error
}

func scanPayment(row paymentScanner) (Payment, error) {
	var payment Payment
	var rawPayload []byte
	err := row.Scan(
		&payment.ID,
		&payment.OrderID,
		&payment.UserID,
		&payment.Provider,
		&payment.MerchantOrderNo,
		&payment.ProviderTradeNo,
		&payment.AmountCents,
		&payment.Currency,
		&payment.Status,
		&rawPayload,
		&payment.PaidAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		return Payment{}, err
	}
	if len(rawPayload) > 0 {
		_ = json.Unmarshal(rawPayload, &payment.RawPayload)
	}
	if payment.RawPayload == nil {
		payment.RawPayload = map[string]any{}
	}
	return payment, nil
}
