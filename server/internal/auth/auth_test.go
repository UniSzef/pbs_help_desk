package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pbs_help_desk/internal/models"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "super-secret-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" || hash == password {
		t.Fatalf("expected non-empty bcrypt hash, got %q", hash)
	}

	if !CheckPassword(password, hash) {
		t.Fatalf("expected CheckPassword to succeed for valid password")
	}
	if CheckPassword("wrong-password", hash) {
		t.Fatalf("expected CheckPassword to fail for wrong password")
	}
}

func TestTokenManager_AccessTokenLifecycle(t *testing.T) {
	tm := NewTokenManager("test-secret-key-1234", 15*time.Minute, 24*time.Hour)
	user := models.User{
		ID:    "user-uuid-123",
		Email: "user@pbs.edu.pl",
		Role:  models.RoleAgent,
	}

	tokenStr, expiresAt, err := tm.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}
	if tokenStr == "" {
		t.Fatalf("expected non-empty token string")
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatalf("expected future expiration time, got %v", expiresAt)
	}

	claims, err := tm.VerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyAccessToken failed: %v", err)
	}
	if claims.UserID != user.ID || claims.Email != user.Email || claims.Role != user.Role {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	otherTM := NewTokenManager("different-secret-1234", 15*time.Minute, 24*time.Hour)
	if _, err := otherTM.VerifyAccessToken(tokenStr); err == nil {
		t.Fatalf("expected error when verifying with wrong secret")
	}

	expiredTM := NewTokenManager("test-secret-key-1234", -1*time.Minute, 24*time.Hour)
	expiredToken, _, err := expiredTM.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}
	if _, err := tm.VerifyAccessToken(expiredToken); err == nil {
		t.Fatalf("expected error when verifying expired token")
	}
}

func TestTokenManager_RefreshTokenAndHash(t *testing.T) {
	tm := NewTokenManager("test-secret-key-1234", 15*time.Minute, 48*time.Hour)

	raw1, hash1, exp1, err := tm.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	raw2, hash2, _, err := tm.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken second call failed: %v", err)
	}

	if raw1 == "" || hash1 == "" || raw1 == hash1 {
		t.Fatalf("invalid raw/hash values: raw=%q hash=%q", raw1, hash1)
	}
	if raw1 == raw2 || hash1 == hash2 {
		t.Fatalf("expected unique random refresh tokens")
	}
	if HashToken(raw1) != hash1 {
		t.Fatalf("expected deterministic HashToken matching generated hash")
	}
	if time.Until(exp1) < 47*time.Hour {
		t.Fatalf("expected refresh token expiration ~48h in future, got %v", exp1)
	}
}

func TestMiddleware_And_ContextHelpers(t *testing.T) {
	tm := NewTokenManager("test-secret-key-1234", 15*time.Minute, 24*time.Hour)
	user := models.User{ID: "user-abc-999", Email: "test@pbs.edu.pl", Role: models.RoleUser}

	validToken, _, err := tm.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	var capturedUserID string
	var capturedRole models.Role
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserIDFromContext(r.Context())
		capturedRole = RoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	protected := Middleware(tm)(nextHandler)

	// 1. Missing Authorization header and cookie -> 401
	req1 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec1 := httptest.NewRecorder()
	protected.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d", rec1.Code)
	}

	// 2. Non-Bearer header -> 401
	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req2.Header.Set("Authorization", "Basic abcdef")
	rec2 := httptest.NewRecorder()
	protected.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-Bearer header, got %d", rec2.Code)
	}

	// 3. Valid Bearer token -> 200
	req3 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req3.Header.Set("Authorization", "Bearer "+validToken)
	rec3 := httptest.NewRecorder()
	protected.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid Bearer token, got %d", rec3.Code)
	}
	if capturedUserID != user.ID || capturedRole != user.Role {
		t.Fatalf("unexpected context values: id=%q role=%q", capturedUserID, capturedRole)
	}

	// 4. Valid access_token cookie -> 200
	capturedUserID = ""
	capturedRole = ""
	req4 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req4.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: validToken})
	rec4 := httptest.NewRecorder()
	protected.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid access_token cookie, got %d", rec4.Code)
	}
	if capturedUserID != user.ID || capturedRole != user.Role {
		t.Fatalf("unexpected context values from cookie: id=%q role=%q", capturedUserID, capturedRole)
	}

	// 5. RequireRole forbids user when admin is required
	adminOnly := Middleware(tm)(RequireRole(models.RoleAdmin)(nextHandler))
	req5 := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req5.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: validToken})
	rec5 := httptest.NewRecorder()
	adminOnly.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for insufficient role, got %d", rec5.Code)
	}
}
