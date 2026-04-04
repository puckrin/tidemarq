// Package crypt provides AES-256-GCM encryption helpers for storing sensitive
// configuration (mount credentials, notification passwords) at rest.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// ErrDecrypt is returned when decryption fails (wrong key or corrupted data).
var ErrDecrypt = errors.New("decryption failed")

// KeyFromSecret derives a 32-byte AES-256 key from an arbitrary secret string.
func KeyFromSecret(secret string) [32]byte {
	return sha256.Sum256([]byte(secret))
}

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
// The returned ciphertext is: nonce (12 bytes) || AES-GCM ciphertext.
func Encrypt(key [32]byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
func Decrypt(key [32]byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecrypt
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
