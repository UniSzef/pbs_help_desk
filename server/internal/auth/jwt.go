package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pbs_help_desk/internal/models"
)

// TokenManager issues and verifies JWT access tokens and opaque refresh tokens.
type TokenManager struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// Claims represents the JWT payload for authenticated users.
type Claims struct {
	UserID string      `json:"uid"`
	Email  string      `json:"email"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

// NewTokenManager constructs a TokenManager with the provided HMAC secret and token lifetimes.
func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:          []byte(secret),
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}
}

// GenerateAccessToken signs an HS256 JWT access token for the given user.
func (m *TokenManager) GenerateAccessToken(user models.User) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTokenTTL)
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// VerifyAccessToken validates a JWT access token string and returns its claims.
func (m *TokenManager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method", models.ErrUnauthorized)
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", models.ErrUnauthorized, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.UserID == "" {
		return nil, models.ErrUnauthorized
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token and its SHA-256 hash.
func (m *TokenManager) GenerateRefreshToken() (rawToken string, tokenHash string, expiresAt time.Time, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", time.Time{}, fmt.Errorf("read random bytes: %w", err)
	}
	rawToken = hex.EncodeToString(buf)
	tokenHash = HashToken(rawToken)
	expiresAt = time.Now().UTC().Add(m.refreshTokenTTL)
	return rawToken, tokenHash, expiresAt, nil
}

// HashToken hashes a raw token string with SHA-256 for safe database storage.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
