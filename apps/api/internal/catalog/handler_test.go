package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestListProductsOnlyReturnsPublishedProducts(t *testing.T) {
	store := newMemoryStore()
	_, _ = store.CreateProduct(context.Background(), sampleProduct("draft-product", StatusDraft))
	_, _ = store.CreateProduct(context.Background(), sampleProduct("published-product", StatusPublished))
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	rec := httptest.NewRecorder()

	handler.ListProducts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body productsResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Products) != 1 || body.Products[0].Slug != "published-product" {
		t.Fatalf("products = %+v, want only published product", body.Products)
	}
}

func TestCreateProductValidatesRequiredFields(t *testing.T) {
	handler := NewHandler(newMemoryStore())
	req := httptest.NewRequest(http.MethodPost, "/api/admin/products", bytes.NewBufferString(`{"name":"","slug":"","priceCents":-1}`))
	rec := httptest.NewRecorder()

	handler.CreateProduct(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateProductReturnsConflictForDuplicateSlug(t *testing.T) {
	store := newMemoryStore()
	_, _ = store.CreateProduct(context.Background(), sampleProduct("product-a", StatusPublished))
	handler := NewHandler(store)

	body := `{"name":"Product A","slug":"product-a","description":"A","priceCents":1000,"currency":"CNY","status":"published","fulfillmentStrategy":"manual_contact"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/products", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.CreateProduct(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

type memoryStore struct {
	mu       sync.Mutex
	nextID   int64
	products map[int64]Product
	slugs    map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		nextID:   1,
		products: map[int64]Product{},
		slugs:    map[string]int64{},
	}
}

func sampleProduct(slug string, status string) CreateProductParams {
	return CreateProductParams{
		Name:                "Product " + slug,
		Slug:                slug,
		Description:         "A product",
		PriceCents:          1200,
		Currency:            "CNY",
		Status:              status,
		FulfillmentStrategy: FulfillmentManualContact,
	}
}

func (s *memoryStore) ListPublishedProducts(_ context.Context) ([]Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	products := []Product{}
	for _, product := range s.products {
		if product.Status == StatusPublished {
			products = append(products, product)
		}
	}
	return products, nil
}

func (s *memoryStore) FindPublishedProductBySlug(_ context.Context, slug string) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.slugs[normalizeSlug(slug)]
	if !ok || s.products[id].Status != StatusPublished {
		return Product{}, ErrProductNotFound
	}
	return s.products[id], nil
}

func (s *memoryStore) ListAdminProducts(_ context.Context) ([]Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	products := []Product{}
	for _, product := range s.products {
		products = append(products, product)
	}
	return products, nil
}

func (s *memoryStore) CreateProduct(_ context.Context, params CreateProductParams) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	slug := normalizeSlug(params.Slug)
	if _, ok := s.slugs[slug]; ok {
		return Product{}, ErrProductSlugExists
	}

	product := Product{
		ID:                  s.nextID,
		Name:                params.Name,
		Slug:                slug,
		Description:         params.Description,
		PriceCents:          params.PriceCents,
		Currency:            normalizeCurrency(params.Currency),
		Status:              params.Status,
		FulfillmentStrategy: params.FulfillmentStrategy,
		ImageURL:            params.ImageURL,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	s.nextID++
	s.products[product.ID] = product
	s.slugs[product.Slug] = product.ID
	return product, nil
}

func (s *memoryStore) UpdateProduct(_ context.Context, params UpdateProductParams) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[params.ID]
	if !ok {
		return Product{}, ErrProductNotFound
	}
	slug := normalizeSlug(params.Slug)
	if existingID, ok := s.slugs[slug]; ok && existingID != params.ID {
		return Product{}, ErrProductSlugExists
	}

	delete(s.slugs, product.Slug)
	product.Name = params.Name
	product.Slug = slug
	product.Description = params.Description
	product.PriceCents = params.PriceCents
	product.Currency = normalizeCurrency(params.Currency)
	product.Status = params.Status
	product.FulfillmentStrategy = params.FulfillmentStrategy
	product.ImageURL = params.ImageURL
	product.UpdatedAt = time.Now()
	s.products[product.ID] = product
	s.slugs[product.Slug] = product.ID
	return product, nil
}
