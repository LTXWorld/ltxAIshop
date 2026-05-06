package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/auth"
	"github.com/ltxai/shop/apps/api/internal/catalog"
	"github.com/ltxai/shop/apps/api/internal/health"
)

type RouterOptions struct {
	Auth    *auth.Handler
	Catalog *catalog.Handler
}

type Option func(*RouterOptions)

func WithAuth(handler auth.Handler) Option {
	return func(options *RouterOptions) {
		options.Auth = &handler
	}
}

func WithCatalog(handler catalog.Handler) Option {
	return func(options *RouterOptions) {
		options.Catalog = &handler
	}
}

func NewRouter(options ...Option) http.Handler {
	var routerOptions RouterOptions
	for _, option := range options {
		option(&routerOptions)
	}

	r := chi.NewRouter()
	r.Get("/api/healthz", health.Handler().ServeHTTP)

	if routerOptions.Catalog != nil {
		catalogHandler := routerOptions.Catalog
		r.Get("/api/products", catalogHandler.ListProducts)
		r.Get("/api/products/{slug}", catalogHandler.GetProduct)
	}

	if routerOptions.Auth != nil {
		authHandler := routerOptions.Auth
		r.Post("/api/auth/register", authHandler.Register)
		r.Post("/api/auth/login", authHandler.Login)
		r.With(authHandler.Middleware).Get("/api/me", authHandler.Me)

		if routerOptions.Catalog != nil {
			catalogHandler := routerOptions.Catalog
			r.With(authHandler.Middleware, auth.RequireAdmin).Get("/api/admin/products", catalogHandler.ListAdminProducts)
			r.With(authHandler.Middleware, auth.RequireAdmin).Post("/api/admin/products", catalogHandler.CreateProduct)
			r.With(authHandler.Middleware, auth.RequireAdmin).Put("/api/admin/products/{id}", catalogHandler.UpdateProduct)
		}
	}

	return r
}
