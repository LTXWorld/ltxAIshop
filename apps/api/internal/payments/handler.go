package payments

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

func (h Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	var req createPaymentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.OrderID <= 0 {
		writeError(w, http.StatusBadRequest, "valid orderId is required")
		return
	}

	payment, err := h.store.CreatePayment(r.Context(), CreatePaymentParams{
		UserID:   userID,
		OrderID:  req.OrderID,
		Provider: req.Provider,
	})
	if errors.Is(err, ErrOrderNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if errors.Is(err, ErrOrderNotPayable) {
		writeError(w, http.StatusConflict, "order is not payable")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create payment failed")
		return
	}

	writeJSON(w, http.StatusCreated, paymentResponseFromPayment(payment))
}

func (h Handler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	paymentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || paymentID <= 0 {
		writeError(w, http.StatusBadRequest, "valid payment id is required")
		return
	}

	var req confirmPaymentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AmountCents < 0 {
		writeError(w, http.StatusBadRequest, "amountCents must be non-negative")
		return
	}

	payment, err := h.store.ConfirmPayment(r.Context(), ConfirmPaymentParams{
		UserID:          userID,
		PaymentID:       paymentID,
		AmountCents:     req.AmountCents,
		ProviderTradeNo: req.ProviderTradeNo,
		RawPayload:      req.RawPayload,
	})
	if errors.Is(err, ErrPaymentNotFound) {
		writeError(w, http.StatusNotFound, "payment not found")
		return
	}
	if errors.Is(err, ErrPaymentAmountMismatch) {
		writeError(w, http.StatusBadRequest, "payment amount mismatch")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "confirm payment failed")
		return
	}

	writeJSON(w, http.StatusOK, paymentResponseFromPayment(payment))
}

func (h Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	paymentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || paymentID <= 0 {
		writeError(w, http.StatusBadRequest, "valid payment id is required")
		return
	}

	payment, err := h.store.FindPayment(r.Context(), userID, paymentID)
	if errors.Is(err, ErrPaymentNotFound) {
		writeError(w, http.StatusNotFound, "payment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load payment failed")
		return
	}
	writeJSON(w, http.StatusOK, paymentResponseFromPayment(payment))
}

func currentUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	return claims.UserID, true
}

type createPaymentRequest struct {
	OrderID  int64  `json:"orderId"`
	Provider string `json:"provider"`
}

type confirmPaymentRequest struct {
	AmountCents     int64          `json:"amountCents"`
	ProviderTradeNo string         `json:"providerTradeNo"`
	RawPayload      map[string]any `json:"rawPayload"`
}

type paymentResponse struct {
	ID              int64          `json:"id"`
	OrderID         int64          `json:"orderId"`
	Provider        string         `json:"provider"`
	MerchantOrderNo string         `json:"merchantOrderNo"`
	ProviderTradeNo string         `json:"providerTradeNo"`
	AmountCents     int64          `json:"amountCents"`
	Currency        string         `json:"currency"`
	Status          string         `json:"status"`
	RawPayload      map[string]any `json:"rawPayload,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func paymentResponseFromPayment(payment Payment) paymentResponse {
	return paymentResponse{
		ID:              payment.ID,
		OrderID:         payment.OrderID,
		Provider:        payment.Provider,
		MerchantOrderNo: payment.MerchantOrderNo,
		ProviderTradeNo: payment.ProviderTradeNo,
		AmountCents:     payment.AmountCents,
		Currency:        payment.Currency,
		Status:          payment.Status,
		RawPayload:      payment.RawPayload,
	}
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
