package config

import (
	"os"
	"path/filepath"
	"testing"
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

	weakSecret := valid
	weakSecret.JWTSecret = "short"
	if err := weakSecret.Validate(); err == nil {
		t.Fatal("expected error for weak JWTSecret")
	}

	placeholder := valid
	placeholder.JWTSecret = "replace-this-with-a-strong-random-secret-key"
	if err := placeholder.Validate(); err == nil {
		t.Fatal("expected error for default placeholder JWTSecret")
	}
}

func TestLoadDotEnv_PreservesDollarAndHashInValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "TEST_DB_URL=postgresql://postgres:p$ss#word%12@localhost:5432/postgres\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp env: %v", err)
	}

	t.Setenv("TEST_DB_URL", "")
	_ = os.Unsetenv("TEST_DB_URL")

	loadDotEnv(envPath)

	got := os.Getenv("TEST_DB_URL")
	want := "postgresql://postgres:p$ss#word%12@localhost:5432/postgres"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
