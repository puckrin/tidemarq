package auth_test

import (
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/auth"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	if !auth.CheckPassword(hash, "correct-horse-battery-staple") {
		t.Error("CheckPassword: expected true for correct password")
	}
	if auth.CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword: expected false for wrong password")
	}
}

func TestIssueAndValidateToken(t *testing.T) {
	svc := auth.NewService("test-secret", time.Hour)

	token, err := svc.IssueToken(42, "alice", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID: got %d, want 42", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Errorf("Username: got %q, want alice", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("Role: got %q, want admin", claims.Role)
	}
}

func TestValidateToken_Tampered(t *testing.T) {
	svc := auth.NewService("test-secret", time.Hour)

	token, err := svc.IssueToken(1, "alice", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	// Append junk to corrupt the signature.
	if _, err := svc.ValidateToken(token + "tampered"); err == nil {
		t.Error("expected error for tampered token")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	svc1 := auth.NewService("secret-a", time.Hour)
	svc2 := auth.NewService("secret-b", time.Hour)

	token, err := svc1.IssueToken(1, "alice", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := svc2.ValidateToken(token); err == nil {
		t.Error("expected error when validating with wrong secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	svc := auth.NewService("test-secret", -time.Second) // already expired

	token, err := svc.IssueToken(1, "alice", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := svc.ValidateToken(token); err == nil {
		t.Error("expected error for expired token")
	}
}
