package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ltxai/shop/apps/api/internal/auth"
	"github.com/ltxai/shop/apps/api/internal/catalog"
)

func TestRouterServesHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("body = %q, want health JSON", body)
	}
}

func TestRouterReturnsNotFoundForUnknownAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRouterServesCatalogEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	rec := httptest.NewRecorder()

	NewRouter(WithCatalog(memoryCatalogHandler())).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminCatalogEndpointRequiresAuthentication(t *testing.T) {
	authHandler := auth.NewHandler(memoryAuthStore{}, auth.NewTokenManager("test-secret-key-with-enough-length"))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/products", nil)
	rec := httptest.NewRecorder()

	NewRouter(WithAuth(authHandler), WithCatalog(memoryCatalogHandler())).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminCatalogEndpointRequiresAdminRole(t *testing.T) {
	user := auth.User{ID: 1, Email: "user@example.com", Role: auth.RoleCustomer}
	authStore := memoryAuthStore{user: user}
	authHandler := auth.NewHandler(authStore, auth.NewTokenManager("test-secret-key-with-enough-length"))
	token, err := auth.NewTokenManager("test-secret-key-with-enough-length").Issue(user, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	NewRouter(WithAuth(authHandler), WithCatalog(memoryCatalogHandler())).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func memoryCatalogHandler() catalog.Handler {
	return catalog.NewHandler(catalogStore{})
}

type catalogStore struct{}

func (catalogStore) ListPublishedProducts(context.Context) ([]catalog.Product, error) {
	return []catalog.Product{}, nil
}

func (catalogStore) FindPublishedProductBySlug(context.Context, string) (catalog.Product, error) {
	return catalog.Product{}, catalog.ErrProductNotFound
}

func (catalogStore) ListAdminProducts(context.Context) ([]catalog.Product, error) {
	return []catalog.Product{}, nil
}

func (catalogStore) CreateProduct(context.Context, catalog.CreateProductParams) (catalog.Product, error) {
	return catalog.Product{}, nil
}

func (catalogStore) UpdateProduct(context.Context, catalog.UpdateProductParams) (catalog.Product, error) {
	return catalog.Product{}, nil
}

type memoryAuthStore struct {
	user auth.User
}

func (s memoryAuthStore) CreateUser(context.Context, string, string, string) (auth.User, error) {
	return s.user, nil
}

func (s memoryAuthStore) FindUserByEmail(context.Context, string) (auth.User, error) {
	return s.user, nil
}

func (s memoryAuthStore) FindUserByID(context.Context, int64) (auth.User, error) {
	return s.user, nil
}
