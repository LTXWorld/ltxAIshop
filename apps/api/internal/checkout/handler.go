package checkout

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/auth"
)

type Handler struct {
	store Store
}

func NewHandler(store Store) Handler {
	return Handler{store: store}
}

func (h Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	cart, err := h.store.GetCart(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load cart failed")
		return
	}
	writeJSON(w, http.StatusOK, cartResponseFromCart(cart))
}

func (h Handler) SetCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	var req cartItemRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProductID <= 0 {
		writeError(w, http.StatusBadRequest, "valid productId is required")
		return
	}
	if req.Quantity < 0 {
		writeError(w, http.StatusBadRequest, "quantity must be non-negative")
		return
	}

	cart, err := h.store.SetCartItem(r.Context(), userID, req.ProductID, req.Quantity)
	h.writeCartMutationResponse(w, cart, err)
}

func (h Handler) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	productID, err := strconv.ParseInt(chi.URLParam(r, "productID"), 10, 64)
	if err != nil || productID <= 0 {
		writeError(w, http.StatusBadRequest, "valid product id is required")
		return
	}

	cart, err := h.store.RemoveCartItem(r.Context(), userID, productID)
	h.writeCartMutationResponse(w, cart, err)
}

func (h Handler) CreateOrderFromCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	order, err := h.store.CreateOrderFromCart(r.Context(), userID)
	if errors.Is(err, ErrCartIsEmpty) {
		writeError(w, http.StatusBadRequest, "cart is empty")
		return
	}
	if errors.Is(err, ErrMixedCurrencies) {
		writeError(w, http.StatusBadRequest, "cart contains mixed currencies")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create order failed")
		return
	}
	writeJSON(w, http.StatusCreated, orderResponseFromOrder(order))
}

func (h Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	orders, err := h.store.ListOrders(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list orders failed")
		return
	}
	writeJSON(w, http.StatusOK, ordersResponse{Orders: orderResponsesFromOrders(orders)})
}

func (h Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || orderID <= 0 {
		writeError(w, http.StatusBadRequest, "valid order id is required")
		return
	}

	order, err := h.store.FindOrder(r.Context(), userID, orderID)
	if errors.Is(err, ErrOrderNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load order failed")
		return
	}
	writeJSON(w, http.StatusOK, orderResponseFromOrder(order))
}

func (h Handler) writeCartMutationResponse(w http.ResponseWriter, cart Cart, err error) {
	if errors.Is(err, ErrProductUnavailable) {
		writeError(w, http.StatusNotFound, "product unavailable")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update cart failed")
		return
	}
	writeJSON(w, http.StatusOK, cartResponseFromCart(cart))
}

func currentUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	return claims.UserID, true
}

type cartItemRequest struct {
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

type cartResponse struct {
	ID               int64              `json:"id"`
	Items            []cartItemResponse `json:"items"`
	TotalCents       int64              `json:"totalCents"`
	Currency         string             `json:"currency"`
	ItemCount        int                `json:"itemCount"`
	HasMixedCurrency bool               `json:"hasMixedCurrency"`
}

type cartItemResponse struct {
	ID                  int64  `json:"id"`
	ProductID           int64  `json:"productId"`
	ProductName         string `json:"productName"`
	ProductSlug         string `json:"productSlug"`
	PriceCents          int64  `json:"priceCents"`
	Currency            string `json:"currency"`
	FulfillmentStrategy string `json:"fulfillmentStrategy"`
	Quantity            int    `json:"quantity"`
	LineTotalCents      int64  `json:"lineTotalCents"`
}

type ordersResponse struct {
	Orders []orderResponse `json:"orders"`
}

type orderResponse struct {
	ID         int64               `json:"id"`
	TotalCents int64               `json:"totalCents"`
	Currency   string              `json:"currency"`
	Status     string              `json:"status"`
	Items      []orderItemResponse `json:"items"`
}

type orderItemResponse struct {
	ID                  int64  `json:"id"`
	ProductID           int64  `json:"productId"`
	ProductName         string `json:"productName"`
	ProductSlug         string `json:"productSlug"`
	PriceCents          int64  `json:"priceCents"`
	Currency            string `json:"currency"`
	FulfillmentStrategy string `json:"fulfillmentStrategy"`
	Quantity            int    `json:"quantity"`
	LineTotalCents      int64  `json:"lineTotalCents"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func cartResponseFromCart(cart Cart) cartResponse {
	response := cartResponse{
		ID:    cart.ID,
		Items: make([]cartItemResponse, 0, len(cart.Items)),
	}
	for _, item := range cart.Items {
		if response.Currency == "" {
			response.Currency = item.Currency
		}
		if response.Currency != item.Currency {
			response.HasMixedCurrency = true
		}
		response.TotalCents += item.LineTotalCents
		response.ItemCount += item.Quantity
		response.Items = append(response.Items, cartItemResponse{
			ID:                  item.ID,
			ProductID:           item.ProductID,
			ProductName:         item.ProductName,
			ProductSlug:         item.ProductSlug,
			PriceCents:          item.PriceCents,
			Currency:            item.Currency,
			FulfillmentStrategy: item.FulfillmentStrategy,
			Quantity:            item.Quantity,
			LineTotalCents:      item.LineTotalCents,
		})
	}
	return response
}

func orderResponsesFromOrders(orders []Order) []orderResponse {
	responses := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		responses = append(responses, orderResponseFromOrder(order))
	}
	return responses
}

func orderResponseFromOrder(order Order) orderResponse {
	response := orderResponse{
		ID:         order.ID,
		TotalCents: order.TotalCents,
		Currency:   order.Currency,
		Status:     order.Status,
		Items:      make([]orderItemResponse, 0, len(order.Items)),
	}
	for _, item := range order.Items {
		response.Items = append(response.Items, orderItemResponse{
			ID:                  item.ID,
			ProductID:           item.ProductID,
			ProductName:         item.ProductName,
			ProductSlug:         item.ProductSlug,
			PriceCents:          item.PriceCents,
			Currency:            item.Currency,
			FulfillmentStrategy: item.FulfillmentStrategy,
			Quantity:            item.Quantity,
			LineTotalCents:      item.LineTotalCents,
		})
	}
	return response
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
