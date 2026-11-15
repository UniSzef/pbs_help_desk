package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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
	loadDotEnv(".env")

	port := getEnv("SERVER_PORT", "")
	if port == "" {
		port = getEnv("PORT", "8080")
	}

	allowedOrigins := splitAndTrim(getEnv("ALLOWED_ORIGINS", ""))
	if len(allowedOrigins) == 0 {
		allowedOrigins = splitAndTrim(getEnv("ALLOWED_ORIGIN", ""))
	}

	return Config{
		Host:            getEnv("HOST", ""),
		Port:            port,
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		AccessTokenTTL:  time.Duration(getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL: time.Duration(getEnvInt("REFRESH_TOKEN_TTL_DAYS", 30)) * 24 * time.Hour,
		CookieDomain:    getEnv("COOKIE_DOMAIN", ""),
		AllowedOrigins:  allowedOrigins,
	}
}

// Validate verifies required production/runtime configuration values.
func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL must be set")
	}

	secret := strings.TrimSpace(c.JWTSecret)
	if secret == "" ||
		secret == "dev-secret-change-me-in-production" ||
		secret == "change-this-to-a-long-random-secret" ||
		secret == "replace-this-with-a-strong-random-secret-key" ||
		len(secret) < 16 {
		return fmt.Errorf("JWT_SECRET must be set to a strong secret (at least 16 characters) and not a default placeholder")
	}

	return nil
}

// loadDotEnv reads KEY=VALUE lines literally without expanding $VARS or stripping # inside values.
func loadDotEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "\xef\xbb\xbf")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
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
