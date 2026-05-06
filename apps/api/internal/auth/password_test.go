package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if !VerifyPassword("correct horse battery staple", hash) {
		t.Fatal("VerifyPassword returned false for the original password")
	}
	if VerifyPassword("wrong password", hash) {
		t.Fatal("VerifyPassword returned true for the wrong password")
	}
}

func TestHashPasswordRejectsShortPassword(t *testing.T) {
	_, err := HashPassword("short")
	if err == nil {
		t.Fatal("HashPassword returned nil error, want short password error")
	}
}
