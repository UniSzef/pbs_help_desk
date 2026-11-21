package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pbs_help_desk/internal/auth"
	"pbs_help_desk/internal/models"
	"pbs_help_desk/internal/services"
)

type mockAuthRepo struct {
	usersByEmail  map[string]models.User
	usersByID     map[string]models.User
	refreshTokens map[string]struct {
		userID    string
		expiresAt time.Time
	}
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		usersByEmail: make(map[string]models.User),
		usersByID:    make(map[string]models.User),
		refreshTokens: make(map[string]struct {
			userID    string
			expiresAt time.Time
		}),
	}
}

func (m *mockAuthRepo) CreateUser(_ context.Context, email, passwordHash, firstName, lastName string) (models.User, error) {
	if _, exists := m.usersByEmail[email]; exists {
		return models.User{}, models.ErrAlreadyExists
	}
	now := time.Now().UTC()
	u := models.User{
		ID:           "user-1",
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		Role:         models.RoleUser,
		IsActive:     true,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.usersByEmail[email] = u
	m.usersByID[u.ID] = u
	return u, nil
}

func (m *mockAuthRepo) GetByEmail(_ context.Context, email string) (models.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return models.User{}, models.ErrNotFound
	}
	return u, nil
}

func (m *mockAuthRepo) GetByID(_ context.Context, userID string) (models.User, error) {
	u, ok := m.usersByID[userID]
	if !ok {
		return models.User{}, models.ErrNotFound
	}
	return u, nil
}

func (m *mockAuthRepo) SaveRefreshToken(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	m.refreshTokens[tokenHash] = struct {
		userID    string
		expiresAt time.Time
	}{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *mockAuthRepo) ConsumeRefreshToken(_ context.Context, tokenHash string) (models.User, error) {
	rec, ok := m.refreshTokens[tokenHash]
	if !ok {
		return models.User{}, models.ErrUnauthorized
	}
	delete(m.refreshTokens, tokenHash)
	if time.Now().UTC().After(rec.expiresAt) {
		return models.User{}, models.ErrUnauthorized
	}
	u, ok := m.usersByID[rec.userID]
	if !ok {
		return models.User{}, models.ErrNotFound
	}
	return u, nil
}

func (m *mockAuthRepo) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(m.refreshTokens, tokenHash)
	return nil
}

func TestAuthCookies_SetAndClear(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	now := time.Now().UTC()
	setAuthCookies(rec, req, "", "access.jwt.token", now.Add(15*time.Minute), "refresh.raw.token", now.Add(24*time.Hour))

	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	var foundAccess, foundRefresh bool
	for _, c := range cookies {
		if c.Name == AccessTokenCookieName {
			foundAccess = true
			if c.Value != "access.jwt.token" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteNoneMode {
				t.Fatalf("unexpected access cookie: %+v", c)
			}
		}
		if c.Name == RefreshTokenCookieName {
			foundRefresh = true
			if c.Value != "refresh.raw.token" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteNoneMode {
				t.Fatalf("unexpected refresh cookie: %+v", c)
			}
		}
	}
	if !foundAccess || !foundRefresh {
		t.Fatalf("missing auth cookies: foundAccess=%v foundRefresh=%v", foundAccess, foundRefresh)
	}

	clearRec := httptest.NewRecorder()
	clearAuthCookies(clearRec, req, "")
	for _, c := range clearRec.Result().Cookies() {
		if c.MaxAge != -1 || c.Value != "" {
			t.Fatalf("expected cleared cookie with MaxAge=-1 and empty value, got %+v", c)
		}
	}
}

func TestAuthHandler_FullFlowWithCookies(t *testing.T) {
	repo := newMockAuthRepo()
	tm := auth.NewTokenManager("test-secret-key-1234", 15*time.Minute, 24*time.Hour)
	svc := services.NewAuthService(repo, tm)
	h := NewAuthHandler(svc, "")

	// 1. Register -> 201 + HttpOnly cookies
	regReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"student@pbs.edu.pl","password":"password123","first_name":"Jan","last_name":"Kowalski"}`))
	regRec := httptest.NewRecorder()
	h.Register(regRec, regReq)

	if regRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", regRec.Code, regRec.Body.String())
	}
	if len(regRec.Result().Cookies()) != 2 {
		t.Fatalf("expected 2 auth cookies on register, got %d", len(regRec.Result().Cookies()))
	}

	// 2. Login -> 200 + HttpOnly cookies
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"student@pbs.edu.pl","password":"password123"}`))
	loginRec := httptest.NewRecorder()
	h.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	var accessCookie, refreshCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == AccessTokenCookieName {
			accessCookie = c
		}
		if c.Name == RefreshTokenCookieName {
			refreshCookie = c
		}
	}
	if accessCookie == nil || refreshCookie == nil {
		t.Fatal("expected access_token and refresh_token cookies on login")
	}

	// 3. Me with access_token cookie via auth.Middleware -> 200
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(accessCookie)
	meRec := httptest.NewRecorder()
	auth.Middleware(tm)(http.HandlerFunc(h.Me)).ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /me, got %d: %s", meRec.Code, meRec.Body.String())
	}
	var meUser models.User
	if err := json.Unmarshal(meRec.Body.Bytes(), &meUser); err != nil {
		t.Fatalf("unmarshal /me: %v", err)
	}
	if meUser.Email != "student@pbs.edu.pl" {
		t.Fatalf("unexpected /me user: %+v", meUser)
	}

	// 4. Refresh with refresh_token cookie -> 200 + rotated cookies
	refReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	refReq.AddCookie(refreshCookie)
	refRec := httptest.NewRecorder()
	h.Refresh(refRec, refReq)

	if refRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on refresh, got %d: %s", refRec.Code, refRec.Body.String())
	}

	var newRefreshCookie *http.Cookie
	for _, c := range refRec.Result().Cookies() {
		if c.Name == RefreshTokenCookieName {
			newRefreshCookie = c
		}
	}
	if newRefreshCookie == nil || newRefreshCookie.Value == refreshCookie.Value {
		t.Fatal("expected rotated refresh_token cookie")
	}

	// 5. Logout with new refresh_token cookie -> 204 + cleared cookies
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(newRefreshCookie)
	logoutRec := httptest.NewRecorder()
	h.Logout(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on logout, got %d", logoutRec.Code)
	}
	for _, c := range logoutRec.Result().Cookies() {
		if c.MaxAge != -1 {
			t.Fatalf("expected cleared cookie on logout, got %+v", c)
		}
	}
}
