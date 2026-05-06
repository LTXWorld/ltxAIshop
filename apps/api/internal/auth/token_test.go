package auth

import (
	"testing"
	"time"
)

func TestTokenIssueAndVerify(t *testing.T) {
	manager := NewTokenManager("test-secret-key-with-enough-length")
	manager.now = func() time.Time { return time.Unix(100, 0) }

	token, err := manager.Issue(User{ID: 42, Email: "user@example.com", Role: RoleCustomer}, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.UserID != 42 || claims.Email != "user@example.com" || claims.Role != RoleCustomer {
		t.Fatalf("claims = %+v, want user 42 customer", claims)
	}
}

func TestTokenRejectsExpiredToken(t *testing.T) {
	manager := NewTokenManager("test-secret-key-with-enough-length")
	manager.now = func() time.Time { return time.Unix(100, 0) }

	token, err := manager.Issue(User{ID: 42, Email: "user@example.com", Role: RoleCustomer}, time.Second)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	manager.now = func() time.Time { return time.Unix(102, 0) }
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("Verify returned nil error, want expired token error")
	}
}
