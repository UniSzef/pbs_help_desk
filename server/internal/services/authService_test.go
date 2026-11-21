package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"pbs_help_desk/internal/auth"
	"pbs_help_desk/internal/models"
)

type mockUserRepo struct {
	usersByEmail  map[string]models.User
	usersByID     map[string]models.User
	refreshTokens map[string]struct {
		userID    string
		expiresAt time.Time
	}
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByEmail: make(map[string]models.User),
		usersByID:    make(map[string]models.User),
		refreshTokens: make(map[string]struct {
			userID    string
			expiresAt time.Time
		}),
	}
}

func (m *mockUserRepo) CreateUser(_ context.Context, email, passwordHash, firstName, lastName string) (models.User, error) {
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

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (models.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return models.User{}, models.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, userID string) (models.User, error) {
	u, ok := m.usersByID[userID]
	if !ok {
		return models.User{}, models.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) SaveRefreshToken(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	m.refreshTokens[tokenHash] = struct {
		userID    string
		expiresAt time.Time
	}{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *mockUserRepo) ConsumeRefreshToken(_ context.Context, tokenHash string) (models.User, error) {
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

func (m *mockUserRepo) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(m.refreshTokens, tokenHash)
	return nil
}

func TestAuthService_RegisterLoginRefreshLogout(t *testing.T) {
	repo := newMockUserRepo()
	tm := auth.NewTokenManager("test-secret-key-1234", 15*time.Minute, 24*time.Hour)
	svc := NewAuthService(repo, tm)
	ctx := context.Background()

	// 1. Register
	regRes, err := svc.Register(ctx, RegisterInput{
		Email:     "Jan.Kowalski@pbs.edu.pl",
		Password:  "password123",
		FirstName: "Jan",
		LastName:  "Kowalski",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if regRes.User.Email != "jan.kowalski@pbs.edu.pl" || regRes.User.Role != models.RoleUser {
		t.Fatalf("unexpected registered user: %+v", regRes.User)
	}
	if regRes.AccessToken == "" || regRes.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}

	// 2. Duplicate register -> ErrAlreadyExists
	_, err = svc.Register(ctx, RegisterInput{
		Email:    "jan.kowalski@pbs.edu.pl",
		Password: "password123",
	})
	if !errors.Is(err, models.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}

	// 3. Login with wrong password -> ErrInvalidCredentials
	_, err = svc.Login(ctx, LoginInput{
		Email:    "jan.kowalski@pbs.edu.pl",
		Password: "wrong-password",
	})
	if !errors.Is(err, models.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// 4. Login with valid password
	loginRes, err := svc.Login(ctx, LoginInput{
		Email:    "jan.kowalski@pbs.edu.pl",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// 5. Refresh token rotation
	refreshRes, err := svc.Refresh(ctx, loginRes.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if refreshRes.RefreshToken == loginRes.RefreshToken {
		t.Fatal("expected rotated refresh token")
	}

	// Reusing old refresh token fails
	if _, err := svc.Refresh(ctx, loginRes.RefreshToken); !errors.Is(err, models.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized on reused refresh token, got %v", err)
	}

	// 6. Logout revokes latest refresh token
	if err := svc.Logout(ctx, refreshRes.RefreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	if _, err := svc.Refresh(ctx, refreshRes.RefreshToken); !errors.Is(err, models.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized after logout, got %v", err)
	}
}
