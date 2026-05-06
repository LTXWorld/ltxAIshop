package catalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store Store
}

func NewHandler(store Store) Handler {
	return Handler{store: store}
}

func (h Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.store.ListPublishedProducts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list products failed")
		return
	}
	writeJSON(w, http.StatusOK, productsResponse{Products: productResponsesFromProducts(products)})
}

func (h Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	product, err := h.store.FindPublishedProductBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, ErrProductNotFound) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load product failed")
		return
	}
	writeJSON(w, http.StatusOK, productResponseFromProduct(product))
}

func (h Handler) ListAdminProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.store.ListAdminProducts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list products failed")
		return
	}
	writeJSON(w, http.StatusOK, productsResponse{Products: productResponsesFromProducts(products)})
}

func (h Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	params, ok := req.createParams(w)
	if !ok {
		return
	}

	product, err := h.store.CreateProduct(r.Context(), params)
	h.writeMutationResponse(w, http.StatusCreated, product, err)
}

func (h Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "valid product id is required")
		return
	}

	var req productRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	params, ok := req.updateParams(w, id)
	if !ok {
		return
	}

	product, err := h.store.UpdateProduct(r.Context(), params)
	h.writeMutationResponse(w, http.StatusOK, product, err)
}

func (h Handler) writeMutationResponse(w http.ResponseWriter, status int, product Product, err error) {
	if errors.Is(err, ErrProductNotFound) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	if errors.Is(err, ErrProductSlugExists) {
		writeError(w, http.StatusConflict, "product slug already exists")
		return
	}
	if errors.Is(err, ErrInvalidProductStatus) {
		writeError(w, http.StatusBadRequest, "invalid product field")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save product failed")
		return
	}
	writeJSON(w, status, productResponseFromProduct(product))
}

type productRequest struct {
	Name                string `json:"name"`
	Slug                string `json:"slug"`
	Description         string `json:"description"`
	PriceCents          int64  `json:"priceCents"`
	Currency            string `json:"currency"`
	Status              string `json:"status"`
	FulfillmentStrategy string `json:"fulfillmentStrategy"`
	ImageURL            string `json:"imageUrl"`
}

func (r productRequest) createParams(w http.ResponseWriter) (CreateProductParams, bool) {
	if !r.valid(w) {
		return CreateProductParams{}, false
	}
	return CreateProductParams{
		Name:                strings.TrimSpace(r.Name),
		Slug:                normalizeSlug(r.Slug),
		Description:         strings.TrimSpace(r.Description),
		PriceCents:          r.PriceCents,
		Currency:            normalizeCurrency(r.Currency),
		Status:              r.Status,
		FulfillmentStrategy: r.FulfillmentStrategy,
		ImageURL:            strings.TrimSpace(r.ImageURL),
	}, true
}

func (r productRequest) updateParams(w http.ResponseWriter, id int64) (UpdateProductParams, bool) {
	if !r.valid(w) {
		return UpdateProductParams{}, false
	}
	return UpdateProductParams{
		ID:                  id,
		Name:                strings.TrimSpace(r.Name),
		Slug:                normalizeSlug(r.Slug),
		Description:         strings.TrimSpace(r.Description),
		PriceCents:          r.PriceCents,
		Currency:            normalizeCurrency(r.Currency),
		Status:              r.Status,
		FulfillmentStrategy: r.FulfillmentStrategy,
		ImageURL:            strings.TrimSpace(r.ImageURL),
	}, true
}

func (r productRequest) valid(w http.ResponseWriter) bool {
	if strings.TrimSpace(r.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return false
	}
	if normalizeSlug(r.Slug) == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return false
	}
	if r.PriceCents < 0 {
		writeError(w, http.StatusBadRequest, "priceCents must be non-negative")
		return false
	}
	if !validStatus(r.Status) {
		writeError(w, http.StatusBadRequest, "valid status is required")
		return false
	}
	if !validFulfillmentStrategy(r.FulfillmentStrategy) {
		writeError(w, http.StatusBadRequest, "valid fulfillmentStrategy is required")
		return false
	}
	return true
}

type productsResponse struct {
	Products []productResponse `json:"products"`
}

type productResponse struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Slug                string `json:"slug"`
	Description         string `json:"description"`
	PriceCents          int64  `json:"priceCents"`
	Currency            string `json:"currency"`
	Status              string `json:"status"`
	FulfillmentStrategy string `json:"fulfillmentStrategy"`
	ImageURL            string `json:"imageUrl"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func productResponsesFromProducts(products []Product) []productResponse {
	responses := make([]productResponse, 0, len(products))
	for _, product := range products {
		responses = append(responses, productResponseFromProduct(product))
	}
	return responses
}

func productResponseFromProduct(product Product) productResponse {
	return productResponse{
		ID:                  product.ID,
		Name:                product.Name,
		Slug:                product.Slug,
		Description:         product.Description,
		PriceCents:          product.PriceCents,
		Currency:            product.Currency,
		Status:              product.Status,
		FulfillmentStrategy: product.FulfillmentStrategy,
		ImageURL:            product.ImageURL,
	}
}

func validStatus(status string) bool {
	return status == StatusDraft || status == StatusPublished || status == StatusArchived
}

func validFulfillmentStrategy(strategy string) bool {
	return strategy == FulfillmentDigitalCredentials ||
		strategy == FulfillmentManualContact ||
		strategy == FulfillmentDigitalCode
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
