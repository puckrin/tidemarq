package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

// ClaimsKey is the context key under which validated JWT claims are stored.
const ClaimsKey contextKey = "auth_claims"

// Middleware validates the Bearer token on every request, attaching the
// claims to the request context. Returns 401 if the token is absent or invalid.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w)
			return
		}

		claims, err := s.ValidateToken(strings.TrimPrefix(authHeader, "Bearer "))
		if err != nil {
			writeUnauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that rejects requests whose token role is not
// one of the allowed roles. Must be chained after Middleware.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil || !allowed[claims.Role] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"forbidden","code":"forbidden"}`)) //nolint:errcheck
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFromContext retrieves JWT claims stored by Middleware.
func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(ClaimsKey).(*Claims)
	return c
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","code":"unauthorized"}`)) //nolint:errcheck
}
