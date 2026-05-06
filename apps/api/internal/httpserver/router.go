package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/auth"
	"github.com/ltxai/shop/apps/api/internal/health"
)

type RouterOptions struct {
	Auth *auth.Handler
}

type Option func(*RouterOptions)

func WithAuth(handler auth.Handler) Option {
	return func(options *RouterOptions) {
		options.Auth = &handler
	}
}

func NewRouter(options ...Option) http.Handler {
	var routerOptions RouterOptions
	for _, option := range options {
		option(&routerOptions)
	}

	r := chi.NewRouter()
	r.Get("/api/healthz", health.Handler().ServeHTTP)

	if routerOptions.Auth != nil {
		authHandler := routerOptions.Auth
		r.Post("/api/auth/register", authHandler.Register)
		r.Post("/api/auth/login", authHandler.Login)
		r.With(authHandler.Middleware).Get("/api/me", authHandler.Me)
	}

	return r
}
