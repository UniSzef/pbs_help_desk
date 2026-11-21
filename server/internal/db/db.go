package db

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ConnectAndMigrate opens a pgx connection pool (supporting Supabase poolers) and applies SQL migrations.
func ConnectAndMigrate(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolCfg, err := parsePoolConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, nil
}

func parsePoolConfig(databaseURL string) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		if sanitized, ok := sanitizeDatabaseURL(databaseURL); ok {
			if cfg, retryErr := pgxpool.ParseConfig(sanitized); retryErr == nil {
				poolCfg = cfg
				err = nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Support Supabase Supavisor transaction pooler (port 6543) as well as session/direct connections.
	if strings.Contains(databaseURL, ":6543") || strings.Contains(databaseURL, "pooler.supabase.com") {
		poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	return poolCfg, nil
}

// sanitizeDatabaseURL percent-encodes raw userinfo characters (e.g., '%' in Supabase passwords) when URL parsing fails.
func sanitizeDatabaseURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)

	var scheme string
	switch {
	case strings.HasPrefix(raw, "postgresql://"):
		scheme = "postgresql"
	case strings.HasPrefix(raw, "postgres://"):
		scheme = "postgres"
	default:
		return "", false
	}

	rest := strings.TrimPrefix(raw, scheme+"://")
	atIdx := strings.LastIndex(rest, "@")
	if atIdx <= 0 {
		return "", false
	}

	userinfo := rest[:atIdx]
	target := rest[atIdx+1:]

	user, password, hasPass := strings.Cut(userinfo, ":")
	if !hasPass {
		return "", false
	}

	hostPort := target
	path := ""
	rawQuery := ""

	if qIdx := strings.Index(hostPort, "?"); qIdx >= 0 {
		rawQuery = hostPort[qIdx+1:]
		hostPort = hostPort[:qIdx]
	}
	if slashIdx := strings.Index(hostPort, "/"); slashIdx >= 0 {
		path = hostPort[slashIdx:]
		hostPort = hostPort[:slashIdx]
	}

	u := &url.URL{
		Scheme:   scheme,
		User:     url.UserPassword(user, password),
		Host:     hostPort,
		Path:     path,
		RawQuery: rawQuery,
	}
	return u.String(), true
}

// StartKeepAlive pings the database periodically so Supabase Free Tier projects do not auto-pause.
func StartKeepAlive(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				var one int
				if err := pool.QueryRow(pingCtx, "SELECT 1").Scan(&one); err != nil {
					slog.Warn("db keep-alive ping failed", "error", err)
				}
				cancel()
			}
		}
	}()
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			filenames = append(filenames, entry.Name())
		}
	}
	sort.Strings(filenames)

	for _, name := range filenames {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration tx %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}

		slog.Info("applied database migration", "filename", name)
	}

	return nil
}
