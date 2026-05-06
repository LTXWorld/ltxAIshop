package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/health"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/healthz", health.Handler().ServeHTTP)
	return r
}
