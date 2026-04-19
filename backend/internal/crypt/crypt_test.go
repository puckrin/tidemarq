package crypt_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tidemarq/tidemarq/internal/crypt"
)

func TestRoundtrip(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")
	plaintext := []byte("sensitive credential data")

	ct, err := crypt.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := crypt.Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("roundtrip: got %q, want %q", got, plaintext)
	}
}

func TestRoundtrip_EmptyPlaintext(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")

	ct, err := crypt.Encrypt(key, []byte{})
	if err != nil {
		t.Fatalf("Encrypt empty plaintext: %v", err)
	}

	got, err := crypt.Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt empty plaintext: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("empty plaintext roundtrip: got %q, want empty", got)
	}
}

func TestEncrypt_UniqueNoncePerCall(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")
	plaintext := []byte("same input every time")

	ct1, err := crypt.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt call 1: %v", err)
	}
	ct2, err := crypt.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt call 2: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("Encrypt produced identical ciphertext on two calls: nonce is not randomised")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := crypt.KeyFromSecret("correct-secret")
	key2 := crypt.KeyFromSecret("wrong-secret")

	ct, _ := crypt.Encrypt(key1, []byte("secret"))
	_, err := crypt.Decrypt(key2, ct)

	if !errors.Is(err, crypt.ErrDecrypt) {
		t.Errorf("wrong key: want ErrDecrypt, got %v", err)
	}
}

// TestDecrypt_TamperedCiphertext verifies that GCM's authentication tag detects
// any modification to the ciphertext body after the nonce.
func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")
	ct, _ := crypt.Encrypt(key, []byte("secret data"))

	ct[12] ^= 0xFF // flip a bit past the 12-byte nonce

	_, err := crypt.Decrypt(key, ct)
	if !errors.Is(err, crypt.ErrDecrypt) {
		t.Errorf("tampered ciphertext: want ErrDecrypt, got %v", err)
	}
}

// TestDecrypt_TamperedNonce verifies that modifying the nonce also causes
// authentication failure — the GCM tag covers the nonce during AEAD Open.
func TestDecrypt_TamperedNonce(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")
	ct, _ := crypt.Encrypt(key, []byte("secret data"))

	ct[0] ^= 0xFF // flip a bit in the nonce

	_, err := crypt.Decrypt(key, ct)
	if !errors.Is(err, crypt.ErrDecrypt) {
		t.Errorf("tampered nonce: want ErrDecrypt, got %v", err)
	}
}

func TestDecrypt_TruncatedInput(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")

	_, err := crypt.Decrypt(key, []byte("short")) // < 12-byte nonce
	if !errors.Is(err, crypt.ErrDecrypt) {
		t.Errorf("truncated input: want ErrDecrypt, got %v", err)
	}
}

func TestDecrypt_EmptyInput(t *testing.T) {
	key := crypt.KeyFromSecret("test-secret")

	_, err := crypt.Decrypt(key, []byte{})
	if !errors.Is(err, crypt.ErrDecrypt) {
		t.Errorf("empty input: want ErrDecrypt, got %v", err)
	}
}

func TestKeyFromSecret_Deterministic(t *testing.T) {
	k1 := crypt.KeyFromSecret("my-secret")
	k2 := crypt.KeyFromSecret("my-secret")

	if k1 != k2 {
		t.Error("KeyFromSecret: same input produced different keys")
	}
}

func TestKeyFromSecret_UniquePerSecret(t *testing.T) {
	k1 := crypt.KeyFromSecret("secret-a")
	k2 := crypt.KeyFromSecret("secret-b")

	if k1 == k2 {
		t.Error("KeyFromSecret: different secrets produced the same key")
	}
}
