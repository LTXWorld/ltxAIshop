package payments

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/alipay"
	"github.com/ltxai/shop/apps/api/internal/auth"
)

type Handler struct {
	store  Store
	alipay *alipay.Client
}

func NewHandler(store Store) Handler {
	return Handler{store: store}
}

func NewHandlerWithAlipay(store Store, client *alipay.Client) Handler {
	return Handler{store: store, alipay: client}
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

	response := paymentResponseFromPayment(payment)
	if payment.Provider == ProviderAlipay && h.alipay != nil {
		pagePay, err := h.alipay.PagePay(alipay.PagePayRequest{
			OutTradeNo:  payment.MerchantOrderNo,
			Subject:     "ltxAI Shop Order " + strconv.FormatInt(payment.OrderID, 10),
			TotalAmount: alipay.AmountFromCents(payment.AmountCents),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create alipay request failed")
			return
		}
		response.PaymentForm = pagePay.FormHTML
		response.PaymentURL = pagePay.GatewayURL
	}

	writeJSON(w, http.StatusCreated, response)
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

func (h Handler) AlipayNotify(w http.ResponseWriter, r *http.Request) {
	if h.alipay == nil {
		http.Error(w, "alipay is not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid notify form", http.StatusBadRequest)
		return
	}

	notify, err := h.alipay.ParseNotify(r.PostForm)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	if notify.TradeStatus != alipay.TradeStatusSuccess && notify.TradeStatus != alipay.TradeStatusFinished {
		_, _ = w.Write([]byte("success"))
		return
	}

	amountCents, err := alipay.CentsFromAmount(notify.TotalAmount)
	if err != nil {
		http.Error(w, "invalid total amount", http.StatusBadRequest)
		return
	}

	_, err = h.store.ConfirmProviderPayment(r.Context(), notify.OutTradeNo, amountCents, notify.TradeNo, valuesToPayload(notify.Raw))
	if errors.Is(err, ErrPaymentNotFound) || errors.Is(err, ErrPaymentAmountMismatch) {
		http.Error(w, "payment validation failed", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "confirm payment failed", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write([]byte("success"))
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
	PaymentURL      string         `json:"paymentUrl,omitempty"`
	PaymentForm     string         `json:"paymentForm,omitempty"`
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

func valuesToPayload(values map[string][]string) map[string]any {
	payload := make(map[string]any, len(values))
	for key, values := range values {
		if len(values) == 1 {
			payload[key] = values[0]
			continue
		}
		payload[key] = values
	}
	return payload
}
