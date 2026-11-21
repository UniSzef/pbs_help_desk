package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestParsePoolConfig_HandlesUnencodedPasswordChars(t *testing.T) {
	tests := []struct {
		name         string
		dsn          string
		wantPassword string
		wantHost     string
		wantSimple   bool
	}{
		{
			name:         "standard url",
			dsn:          "postgresql://postgres:secret123@localhost:5432/postgres?sslmode=disable",
			wantPassword: "secret123",
			wantHost:     "localhost",
			wantSimple:   false,
		},
		{
			name:         "already percent-encoded password",
			dsn:          "postgresql://postgres:my%25pass@aws-0-eu-west-2.pooler.supabase.com:5432/postgres",
			wantPassword: "my%pass",
			wantHost:     "aws-0-eu-west-2.pooler.supabase.com",
			wantSimple:   true,
		},
		{
			name:         "unencoded percent and special characters in supabase password",
			dsn:          "postgresql://postgres.ref:abc%xyz#1@2/3?4@aws-0-eu-west-2.pooler.supabase.com:6543/postgres?sslmode=require",
			wantPassword: "abc%xyz#1@2/3?4",
			wantHost:     "aws-0-eu-west-2.pooler.supabase.com",
			wantSimple:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parsePoolConfig(tc.dsn)
			if err != nil {
				t.Fatalf("parsePoolConfig failed: %v", err)
			}
			if cfg.ConnConfig.Password != tc.wantPassword {
				t.Errorf("expected password %q, got %q", tc.wantPassword, cfg.ConnConfig.Password)
			}
			if cfg.ConnConfig.Host != tc.wantHost {
				t.Errorf("expected host %q, got %q", tc.wantHost, cfg.ConnConfig.Host)
			}
			isSimple := cfg.ConnConfig.DefaultQueryExecMode == pgx.QueryExecModeSimpleProtocol
			if isSimple != tc.wantSimple {
				t.Errorf("expected simple protocol=%v, got %v", tc.wantSimple, isSimple)
			}
		})
	}
}
