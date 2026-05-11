package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 50
	cfg.MinConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{Pool: pool}
	if err := db.migrate(ctx); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("database connected and migrated", "max_conns", cfg.MaxConns)
	return db, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}
