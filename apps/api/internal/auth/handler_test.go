package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRegisterCreatesCustomerAndReturnsToken(t *testing.T) {
	store := newMemoryStore()
	handler := NewHandler(store, NewTokenManager("test-secret-key-with-enough-length"))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"USER@example.com","password":"correct horse"}`))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var body authResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Token == "" {
		t.Fatal("token should be set")
	}
	if body.User.Email != "user@example.com" || body.User.Role != RoleCustomer {
		t.Fatalf("user = %+v, want normalized customer", body.User)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	store := newMemoryStore()
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if _, err := store.CreateUser(context.Background(), "user@example.com", hash, RoleCustomer); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	handler := NewHandler(store, NewTokenManager("test-secret-key-with-enough-length"))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"user@example.com","password":"wrong password"}`))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeRequiresBearerToken(t *testing.T) {
	store := newMemoryStore()
	handler := NewHandler(store, NewTokenManager("test-secret-key-with-enough-length"))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	handler.Middleware(http.HandlerFunc(handler.Me)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeReturnsCurrentUser(t *testing.T) {
	store := newMemoryStore()
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	user, err := store.CreateUser(context.Background(), "user@example.com", hash, RoleCustomer)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	tokens := NewTokenManager("test-secret-key-with-enough-length")
	token, err := tokens.Issue(user, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	handler := NewHandler(store, tokens)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.Middleware(http.HandlerFunc(handler.Me)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

type memoryStore struct {
	mu     sync.Mutex
	nextID int64
	users  map[int64]User
	emails map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		nextID: 1,
		users:  map[int64]User{},
		emails: map[string]int64{},
	}
}

func (s *memoryStore) CreateUser(_ context.Context, email string, passwordHash string, role string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email = normalizeEmail(email)
	if _, ok := s.emails[email]; ok {
		return User{}, ErrEmailAlreadyRegistered
	}

	user := User{
		ID:           s.nextID,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	s.nextID++
	s.users[user.ID] = user
	s.emails[user.Email] = user.ID
	return user, nil
}

func (s *memoryStore) FindUserByEmail(_ context.Context, email string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.emails[normalizeEmail(email)]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return s.users[id], nil
}

func (s *memoryStore) FindUserByID(_ context.Context, id int64) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	if user.ID == 0 {
		return User{}, errors.New("invalid fixture")
	}
	return user, nil
}
