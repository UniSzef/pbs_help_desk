package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"pbs_help_desk/internal/models"
)

type contextKey string

const (
	userClaimsKey        contextKey = "user_claims"
	AccessTokenCookieName           = "access_token"
)

// Middleware authenticates requests via Authorization Bearer header or access_token HttpOnly cookie.
func Middleware(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			} else if cookie, err := r.Cookie(AccessTokenCookieName); err == nil && cookie.Value != "" {
				tokenStr = strings.TrimSpace(cookie.Value)
			}

			if tokenStr == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing or invalid authorization header or cookie")
				return
			}

			claims, err := tm.VerifyAccessToken(tokenStr)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired access token")
				return
			}

			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole restricts access to users holding one of the allowed roles.
func RequireRole(roles ...models.Role) func(http.Handler) http.Handler {
	allowed := make(map[models.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				writeAuthError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				writeAuthError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFromContext extracts the authenticated user's JWT claims from context.
func ClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(userClaimsKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// UserIDFromContext extracts the authenticated user's ID from context.
func UserIDFromContext(ctx context.Context) string {
	claims := ClaimsFromContext(ctx)
	if claims == nil {
		return ""
	}
	return claims.UserID
}

// RoleFromContext extracts the authenticated user's role from context.
func RoleFromContext(ctx context.Context) models.Role {
	claims := ClaimsFromContext(ctx)
	if claims == nil {
		return ""
	}
	return claims.Role
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
