package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"pbs_help_desk/internal/auth"
	"pbs_help_desk/internal/models"
	"pbs_help_desk/internal/services"
)

// AuthHandler handles registration, login, token refresh, logout, and current-user endpoints.
type AuthHandler struct {
	service      *services.AuthService
	cookieDomain string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(service *services.AuthService, cookieDomain string) *AuthHandler {
	return &AuthHandler{
		service:      service,
		cookieDomain: cookieDomain,
	}
}

// Register creates a new user account and sets HttpOnly JWT cookies.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req services.RegisterInput
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON payload", models.ErrValidation))
		return
	}

	result, err := h.service.Register(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	setAuthCookies(w, r, h.cookieDomain, result.AccessToken, result.ExpiresAt, result.RefreshToken, result.RefreshExpiresAt)
	writeJSON(w, http.StatusCreated, result)
}

// Login authenticates an existing user and sets HttpOnly JWT cookies.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req services.LoginInput
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON payload", models.ErrValidation))
		return
	}

	result, err := h.service.Login(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	setAuthCookies(w, r, h.cookieDomain, result.AccessToken, result.ExpiresAt, result.RefreshToken, result.RefreshExpiresAt)
	writeJSON(w, http.StatusOK, result)
}

// Refresh rotates the refresh token and issues a fresh access/refresh token pair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken := extractRefreshToken(w, r)

	result, err := h.service.Refresh(r.Context(), rawRefreshToken)
	if err != nil {
		clearAuthCookies(w, r, h.cookieDomain)
		writeError(w, err)
		return
	}

	setAuthCookies(w, r, h.cookieDomain, result.AccessToken, result.ExpiresAt, result.RefreshToken, result.RefreshExpiresAt)
	writeJSON(w, http.StatusOK, result)
}

// Logout revokes the refresh token (if provided) and clears auth cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	rawRefreshToken := extractRefreshToken(w, r)
	_ = h.service.Logout(r.Context(), rawRefreshToken)

	clearAuthCookies(w, r, h.cookieDomain)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the profile of the currently authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	user, err := h.service.Me(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func extractRefreshToken(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(RefreshTokenCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}

	if r.Body != nil {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := decodeJSON(w, r, &req); err == nil {
			return strings.TrimSpace(req.RefreshToken)
		}
	}

	return ""
}
