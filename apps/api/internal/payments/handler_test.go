package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/auth"
)

func TestCreatePaymentReturnsExistingPaymentForOrder(t *testing.T) {
	store := newMemoryStore()
	handler := NewHandler(store)

	req := authenticatedRequest(http.MethodPost, "/api/payments", `{"orderId":1,"provider":"manual"}`, 10)
	rec := httptest.NewRecorder()
	handler.CreatePayment(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var first paymentResponse
	if err := json.NewDecoder(rec.Body).Decode(&first); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	req = authenticatedRequest(http.MethodPost, "/api/payments", `{"orderId":1,"provider":"manual"}`, 10)
	rec = httptest.NewRecorder()
	handler.CreatePayment(rec, req)

	var second paymentResponse
	if err := json.NewDecoder(rec.Body).Decode(&second); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("second payment ID = %d, want existing payment ID %d", second.ID, first.ID)
	}
}

func TestConfirmPaymentMarksSucceeded(t *testing.T) {
	store := newMemoryStore()
	payment, err := store.CreatePayment(context.Background(), CreatePaymentParams{UserID: 10, OrderID: 1, Provider: ProviderManual})
	if err != nil {
		t.Fatalf("CreatePayment returned error: %v", err)
	}
	handler := NewHandler(store)

	req := authenticatedRequest(http.MethodPost, "/api/payments/1/confirm", `{"amountCents":19800,"providerTradeNo":"manual-1","rawPayload":{"source":"test"}}`, 10)
	req = withPathParam(req, "id", payment.ID)
	rec := httptest.NewRecorder()

	handler.ConfirmPayment(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body paymentResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != StatusSucceeded || body.ProviderTradeNo != "manual-1" {
		t.Fatalf("payment = %+v, want succeeded manual-1", body)
	}

	order := store.orders[1]
	if order.Status != OrderStatusPaid {
		t.Fatalf("order status = %q, want paid", order.Status)
	}
}

func TestConfirmPaymentRejectsAmountMismatch(t *testing.T) {
	store := newMemoryStore()
	payment, _ := store.CreatePayment(context.Background(), CreatePaymentParams{UserID: 10, OrderID: 1, Provider: ProviderManual})
	handler := NewHandler(store)

	req := authenticatedRequest(http.MethodPost, "/api/payments/1/confirm", `{"amountCents":1}`, 10)
	req = withPathParam(req, "id", payment.ID)
	rec := httptest.NewRecorder()

	handler.ConfirmPayment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func authenticatedRequest(method string, path string, body string, userID int64) *http.Request {
	var reader *bytes.Buffer
	if body == "" {
		reader = bytes.NewBuffer(nil)
	} else {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	ctx := auth.ContextWithClaims(req.Context(), auth.Claims{
		UserID: userID,
		Email:  "user@example.com",
		Role:   auth.RoleCustomer,
		Expiry: time.Now().Add(time.Hour).Unix(),
	})
	return req.WithContext(ctx)
}

func withPathParam(req *http.Request, key string, value int64) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, strconv.FormatInt(value, 10))
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

type memoryStore struct {
	mu       sync.Mutex
	nextID   int64
	orders   map[int64]memoryOrder
	payments map[int64]Payment
}

type memoryOrder struct {
	ID         int64
	UserID     int64
	TotalCents int64
	Currency   string
	Status     string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		nextID: 1,
		orders: map[int64]memoryOrder{
			1: {ID: 1, UserID: 10, TotalCents: 19800, Currency: "CNY", Status: OrderStatusPendingPayment},
		},
		payments: map[int64]Payment{},
	}
}

func (s *memoryStore) CreatePayment(_ context.Context, params CreatePaymentParams) (Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[params.OrderID]
	if !ok || order.UserID != params.UserID {
		return Payment{}, ErrOrderNotFound
	}
	if order.Status != OrderStatusPendingPayment {
		return Payment{}, ErrOrderNotPayable
	}
	for _, payment := range s.payments {
		if payment.OrderID == params.OrderID && payment.UserID == params.UserID {
			return payment, nil
		}
	}

	provider := params.Provider
	if provider == "" {
		provider = ProviderManual
	}
	payment := Payment{
		ID:              s.nextID,
		OrderID:         params.OrderID,
		UserID:          params.UserID,
		Provider:        provider,
		MerchantOrderNo: "LTXTEST",
		AmountCents:     order.TotalCents,
		Currency:        order.Currency,
		Status:          StatusCreated,
		RawPayload:      map[string]any{},
	}
	s.nextID++
	s.payments[payment.ID] = payment
	return payment, nil
}

func (s *memoryStore) ConfirmPayment(_ context.Context, params ConfirmPaymentParams) (Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, ok := s.payments[params.PaymentID]
	if !ok || payment.UserID != params.UserID {
		return Payment{}, ErrPaymentNotFound
	}
	if payment.AmountCents != params.AmountCents {
		return Payment{}, ErrPaymentAmountMismatch
	}
	if payment.Status == StatusSucceeded {
		return payment, nil
	}
	now := time.Now()
	payment.Status = StatusSucceeded
	payment.ProviderTradeNo = params.ProviderTradeNo
	payment.RawPayload = params.RawPayload
	payment.PaidAt = &now
	s.payments[payment.ID] = payment

	order := s.orders[payment.OrderID]
	if order.Status == OrderStatusPendingPayment {
		order.Status = OrderStatusPaid
		s.orders[order.ID] = order
	}
	return payment, nil
}

func (s *memoryStore) FindPayment(_ context.Context, userID int64, paymentID int64) (Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, ok := s.payments[paymentID]
	if !ok || payment.UserID != userID {
		return Payment{}, ErrPaymentNotFound
	}
	return payment, nil
}
