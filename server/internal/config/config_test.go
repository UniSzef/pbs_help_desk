package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestConfig_Validate(t *testing.T) {
	valid := Config{
		DatabaseURL: "postgresql://postgres:pass@localhost:5432/postgres",
		JWTSecret:   "super-strong-secret-key-123456",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	missingDB := valid
	missingDB.DatabaseURL = ""
	if err := missingDB.Validate(); err == nil {
		t.Fatal("expected error for missing DatabaseURL")
	}

	emptySecret := valid
	emptySecret.JWTSecret = ""
	if err := emptySecret.Validate(); err == nil {
		t.Fatal("expected error for empty JWTSecret")
	}

	weakSecret := valid
	weakSecret.JWTSecret = "short"
	if err := weakSecret.Validate(); err == nil {
		t.Fatal("expected error for weak JWTSecret")
	}
}

func TestGodotenv_SingleQuotedPreservesSpecialChars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "TEST_DB_URL='postgresql://postgres:p$ss#word%12@localhost:5432/postgres'\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp env: %v", err)
	}

	t.Setenv("TEST_DB_URL", "")
	_ = os.Unsetenv("TEST_DB_URL")

	if err := godotenv.Load(envPath); err != nil {
		t.Fatalf("godotenv.Load failed: %v", err)
	}

	got := os.Getenv("TEST_DB_URL")
	want := "postgresql://postgres:p$ss#word%12@localhost:5432/postgres"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
