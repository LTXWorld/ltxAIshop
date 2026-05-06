package checkout

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

func TestSetCartItemReturnsCartTotals(t *testing.T) {
	store := newMemoryStore()
	handler := NewHandler(store)

	req := authenticatedRequest(http.MethodPut, "/api/cart/items", `{"productId":1,"quantity":2}`, 10)
	rec := httptest.NewRecorder()

	handler.SetCartItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body cartResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ItemCount != 2 || body.TotalCents != 2400 {
		t.Fatalf("cart = %+v, want 2 items and 2400 total", body)
	}
}

func TestCreateOrderFromCartSnapshotsItemsAndClearsCart(t *testing.T) {
	store := newMemoryStore()
	_, err := store.SetCartItem(context.Background(), 10, 1, 2)
	if err != nil {
		t.Fatalf("SetCartItem returned error: %v", err)
	}
	handler := NewHandler(store)

	req := authenticatedRequest(http.MethodPost, "/api/orders", "", 10)
	rec := httptest.NewRecorder()

	handler.CreateOrderFromCart(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var body orderResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != OrderStatusPendingPayment || body.TotalCents != 2400 || len(body.Items) != 1 {
		t.Fatalf("order = %+v, want pending order with one item", body)
	}

	cart, err := store.GetCart(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetCart returned error: %v", err)
	}
	if len(cart.Items) != 0 {
		t.Fatalf("cart has %d items, want empty cart", len(cart.Items))
	}
}

func TestCreateOrderFromCartRejectsEmptyCart(t *testing.T) {
	handler := NewHandler(newMemoryStore())

	req := authenticatedRequest(http.MethodPost, "/api/orders", "", 10)
	rec := httptest.NewRecorder()

	handler.CreateOrderFromCart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetOrderCannotReadAnotherUsersOrder(t *testing.T) {
	store := newMemoryStore()
	_, _ = store.SetCartItem(context.Background(), 10, 1, 1)
	order, err := store.CreateOrderFromCart(context.Background(), 10)
	if err != nil {
		t.Fatalf("CreateOrderFromCart returned error: %v", err)
	}
	handler := NewHandler(store)

	req := authenticatedRequest(http.MethodGet, "/api/orders/1", "", 11)
	req = withPathParam(req, "id", order.ID)
	rec := httptest.NewRecorder()

	handler.GetOrder(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
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
	nextCart int64
	nextItem int64
	nextOrd  int64
	carts    map[int64]Cart
	orders   map[int64]Order
	products map[int64]OrderItem
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		nextCart: 1,
		nextItem: 1,
		nextOrd:  1,
		carts:    map[int64]Cart{},
		orders:   map[int64]Order{},
		products: map[int64]OrderItem{
			1: {
				ProductID:           1,
				ProductName:         "Product A",
				ProductSlug:         "product-a",
				PriceCents:          1200,
				Currency:            "CNY",
				FulfillmentStrategy: "manual_contact",
			},
		},
	}
}

func (s *memoryStore) GetCart(_ context.Context, userID int64) (Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureCart(userID), nil
}

func (s *memoryStore) SetCartItem(_ context.Context, userID int64, productID int64, quantity int) (Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[productID]
	if !ok {
		return Cart{}, ErrProductUnavailable
	}
	cart := s.ensureCart(userID)
	if quantity <= 0 {
		return s.removeCartItemLocked(userID, productID), nil
	}

	for i := range cart.Items {
		if cart.Items[i].ProductID == productID {
			cart.Items[i].Quantity = quantity
			cart.Items[i].LineTotalCents = cart.Items[i].PriceCents * int64(quantity)
			s.carts[userID] = cart
			return cart, nil
		}
	}

	item := CartItem{
		ID:                  s.nextItem,
		ProductID:           product.ProductID,
		ProductName:         product.ProductName,
		ProductSlug:         product.ProductSlug,
		PriceCents:          product.PriceCents,
		Currency:            product.Currency,
		FulfillmentStrategy: product.FulfillmentStrategy,
		Quantity:            quantity,
		LineTotalCents:      product.PriceCents * int64(quantity),
	}
	s.nextItem++
	cart.Items = append(cart.Items, item)
	s.carts[userID] = cart
	return cart, nil
}

func (s *memoryStore) RemoveCartItem(_ context.Context, userID int64, productID int64) (Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeCartItemLocked(userID, productID), nil
}

func (s *memoryStore) CreateOrderFromCart(_ context.Context, userID int64) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart := s.ensureCart(userID)
	if len(cart.Items) == 0 {
		return Order{}, ErrCartIsEmpty
	}

	order := Order{
		ID:       s.nextOrd,
		UserID:   userID,
		Currency: cart.Items[0].Currency,
		Status:   OrderStatusPendingPayment,
		Items:    []OrderItem{},
	}
	s.nextOrd++
	for _, cartItem := range cart.Items {
		item := OrderItem{
			ID:                  cartItem.ID,
			ProductID:           cartItem.ProductID,
			ProductName:         cartItem.ProductName,
			ProductSlug:         cartItem.ProductSlug,
			PriceCents:          cartItem.PriceCents,
			Currency:            cartItem.Currency,
			FulfillmentStrategy: cartItem.FulfillmentStrategy,
			Quantity:            cartItem.Quantity,
			LineTotalCents:      cartItem.LineTotalCents,
		}
		order.TotalCents += item.LineTotalCents
		order.Items = append(order.Items, item)
	}
	s.orders[order.ID] = order
	cart.Items = []CartItem{}
	s.carts[userID] = cart
	return order, nil
}

func (s *memoryStore) ListOrders(_ context.Context, userID int64) ([]Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	orders := []Order{}
	for _, order := range s.orders {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	return orders, nil
}

func (s *memoryStore) FindOrder(_ context.Context, userID int64, orderID int64) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok || order.UserID != userID {
		return Order{}, ErrOrderNotFound
	}
	return order, nil
}

func (s *memoryStore) ensureCart(userID int64) Cart {
	cart, ok := s.carts[userID]
	if ok {
		return cart
	}
	cart = Cart{ID: s.nextCart, UserID: userID, Items: []CartItem{}}
	s.nextCart++
	s.carts[userID] = cart
	return cart
}

func (s *memoryStore) removeCartItemLocked(userID int64, productID int64) Cart {
	cart := s.ensureCart(userID)
	items := []CartItem{}
	for _, item := range cart.Items {
		if item.ProductID != productID {
			items = append(items, item)
		}
	}
	cart.Items = items
	s.carts[userID] = cart
	return cart
}
