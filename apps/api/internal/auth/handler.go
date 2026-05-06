package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const tokenTTL = 24 * time.Hour

type contextKey string

const claimsContextKey contextKey = "authClaims"

type Handler struct {
	store  Store
	tokens TokenManager
}

func NewHandler(store Store, tokens TokenManager) Handler {
	return Handler{store: store, tokens: tokens}
}

func (h Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	email := normalizeEmail(req.Email)
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.store.CreateUser(r.Context(), email, passwordHash, RoleCustomer)
	if errors.Is(err, ErrEmailAlreadyRegistered) {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create user failed")
		return
	}

	h.writeAuthResponse(w, http.StatusCreated, user)
}

func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.store.FindUserByEmail(r.Context(), req.Email)
	if errors.Is(err, ErrUserNotFound) || !VerifyPassword(req.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load user failed")
		return
	}

	h.writeAuthResponse(w, http.StatusOK, user)
}

func (h Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.store.FindUserByID(r.Context(), claims.UserID)
	if errors.Is(err, ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load user failed")
		return
	}

	writeJSON(w, http.StatusOK, userResponseFromUser(user))
}

func (h Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		claims, err := h.tokens.Verify(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(Claims)
	return claims, ok
}

func (h Handler) writeAuthResponse(w http.ResponseWriter, status int, user User) {
	token, err := h.tokens.Issue(user, tokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "issue token failed")
		return
	}

	writeJSON(w, status, authResponse{
		Token: token,
		User:  userResponseFromUser(user),
	})
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

type userResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func userResponseFromUser(user User) userResponse {
	return userResponse{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Role,
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

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
