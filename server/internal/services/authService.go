package services

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"pbs_help_desk/internal/auth"
	"pbs_help_desk/internal/models"
	"pbs_help_desk/internal/repositories"
)

// RegisterInput holds user registration fields.
type RegisterInput struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// LoginInput holds user login credentials.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResult contains the authenticated user and issued token pair.
type AuthResult struct {
	User             models.User `json:"user"`
	AccessToken      string      `json:"access_token"`
	ExpiresAt        time.Time   `json:"expires_at"`
	RefreshToken     string      `json:"refresh_token"`
	RefreshExpiresAt time.Time   `json:"-"`
}

// AuthService encapsulates registration, login, refresh token rotation, and logout logic.
type AuthService struct {
	users  repositories.UserRepository
	tokens *auth.TokenManager
}

// NewAuthService creates a new AuthService.
func NewAuthService(users repositories.UserRepository, tokens *auth.TokenManager) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
	}
}

// Register validates input, creates a new user account, and issues a token pair.
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return AuthResult{}, fmt.Errorf("%w: valid email is required", models.ErrValidation)
	}
	if len(input.Password) < 8 {
		return AuthResult{}, fmt.Errorf("%w: password must be at least 8 characters", models.ErrValidation)
	}
	if len(input.Password) > 72 {
		return AuthResult{}, fmt.Errorf("%w: password must not exceed 72 bytes", models.ErrValidation)
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return AuthResult{}, err
	}

	user, err := s.users.CreateUser(ctx, email, hash, input.FirstName, input.LastName)
	if err != nil {
		return AuthResult{}, err
	}

	return s.issueTokens(ctx, user)
}

// Login verifies user credentials and issues a new token pair.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	if len(input.Password) > 72 || strings.TrimSpace(input.Email) == "" {
		return AuthResult{}, models.ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, input.Email)
	if err != nil {
		return AuthResult{}, models.ErrInvalidCredentials
	}

	if !user.IsActive {
		return AuthResult{}, models.ErrForbidden
	}

	if !auth.CheckPassword(input.Password, user.PasswordHash) {
		return AuthResult{}, models.ErrInvalidCredentials
	}

	return s.issueTokens(ctx, user)
}

// Refresh consumes a refresh token and rotates both access and refresh tokens.
func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (AuthResult, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return AuthResult{}, fmt.Errorf("%w: refresh_token is required in cookie or body", models.ErrValidation)
	}

	tokenHash := auth.HashToken(rawRefreshToken)
	user, err := s.users.ConsumeRefreshToken(ctx, tokenHash)
	if err != nil {
		return AuthResult{}, err
	}

	if !user.IsActive {
		return AuthResult{}, models.ErrForbidden
	}

	return s.issueTokens(ctx, user)
}

// Logout revokes the provided refresh token if present.
func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return nil
	}
	tokenHash := auth.HashToken(rawRefreshToken)
	return s.users.DeleteRefreshToken(ctx, tokenHash)
}

// Me retrieves the authenticated user's profile.
func (s *AuthService) Me(ctx context.Context, userID string) (models.User, error) {
	if userID == "" {
		return models.User{}, models.ErrUnauthorized
	}
	return s.users.GetByID(ctx, userID)
}

func (s *AuthService) issueTokens(ctx context.Context, user models.User) (AuthResult, error) {
	accessToken, expiresAt, err := s.tokens.GenerateAccessToken(user)
	if err != nil {
		return AuthResult{}, err
	}

	rawRefresh, refreshHash, refreshExpiresAt, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}

	if err := s.users.SaveRefreshToken(ctx, user.ID, refreshHash, refreshExpiresAt); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:             user,
		AccessToken:      accessToken,
		ExpiresAt:        expiresAt,
		RefreshToken:     rawRefresh,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}
