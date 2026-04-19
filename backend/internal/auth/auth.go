package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Claims is the JWT payload.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Service issues and validates JWTs and wraps password hashing.
type Service struct {
	secret []byte
	ttl    time.Duration
}

// NewService creates an auth service with the given HMAC secret and token TTL.
func NewService(secret string, ttl time.Duration) *Service {
	return &Service{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// IssueToken mints a signed JWT for the given user.
func (s *Service) IssueToken(userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// IssueTokenTTL mints a signed JWT with an explicit TTL, overriding the service default.
func (s *Service) IssueTokenTTL(userID int64, username, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// IssueWSToken mints a short-lived JWT for WebSocket authentication. Each call
// embeds a random jti (JWT ID) so tokens issued within the same second for the
// same user are distinct strings — a prerequisite for the nonce-based
// single-use enforcement in the WS handler.
func (s *Service) IssueWSToken(userID int64, username, role string, ttl time.Duration) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating token ID: %w", err)
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(raw),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// ValidateToken parses and validates a JWT string, returning its claims.
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// HashPassword bcrypt-hashes a plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether plaintext matches the bcrypt hash.
func CheckPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

// dummyHash is a pre-computed bcrypt hash used by CheckPasswordDummy.
// It exists solely to equalise login timing when the requested username does
// not exist, preventing statistical timing attacks that distinguish "no such
// user" (~microseconds) from "wrong password" (~300ms bcrypt).
var dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("tidemarq-dummy-password-do-not-use"), bcrypt.DefaultCost)
	return string(h)
}()

// CheckPasswordDummy runs bcrypt against a dummy hash. Call this on the
// username-not-found path to spend the same ~300ms as a real password check.
func CheckPasswordDummy(plaintext string) {
	bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(plaintext)) //nolint:errcheck
}
