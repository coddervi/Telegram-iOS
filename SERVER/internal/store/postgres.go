package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil { return nil, fmt.Errorf("parse database config: %w", err) }
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { return nil, fmt.Errorf("connect database: %w", err) }
	if err := pool.Ping(ctx); err != nil { pool.Close(); return nil, fmt.Errorf("ping database: %w", err) }
	if err := Migrate(ctx, pool); err != nil { pool.Close(); return nil, err }
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil { return err }
	names := make([]string, 0, len(entries))
	for _, entry := range entries { if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") { names = append(names, entry.Name()) } }
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", name).Scan(&exists); err != nil { return err }
		if exists { continue }
		sql, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil { return err }
		tx, err := pool.Begin(ctx)
		if err != nil { return err }
		if _, err = tx.Exec(ctx, string(sql)); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", name)
		}
		if err != nil { _ = tx.Rollback(ctx); return fmt.Errorf("apply migration %s: %w", name, err) }
		if err := tx.Commit(ctx); err != nil { return fmt.Errorf("commit migration %s: %w", name, err) }
	}
	return nil
}
