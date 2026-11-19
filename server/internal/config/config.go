package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Host            string
	Port            string
	DatabaseURL     string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CookieDomain    string
	AllowedOrigins  []string
}

// Load loads configuration from .env (if present) and environment variables.
func Load() Config {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("failed to load .env file", "error", err)
	}

	return Config{
		Host:            getEnv("HOST", ""),
		Port:            getEnv("SERVER_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		AccessTokenTTL:  time.Duration(getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL: time.Duration(getEnvInt("REFRESH_TOKEN_TTL_DAYS", 30)) * 24 * time.Hour,
		CookieDomain:    getEnv("COOKIE_DOMAIN", ""),
		AllowedOrigins:  splitAndTrim(getEnv("ALLOWED_ORIGINS", "")),
	}
}

// Validate verifies required production/runtime configuration values.
func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL must be set")
	}

	if len(strings.TrimSpace(c.JWTSecret)) < 16 {
		return fmt.Errorf("JWT_SECRET must be set to a strong secret (at least 16 characters)")
	}

	return nil
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			res = append(res, t)
		}
	}
	return res
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
