package postgres

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
    Pool *pgxpool.Pool
}

func Connect(ctx context.Context, url string) (*DB, error) {
    cfg, err := pgxpool.ParseConfig(url)
    if err != nil {
        return nil, err
    }
    cfg.MaxConns = 10
    cfg.HealthCheckPeriod = 30 * time.Second
    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, err
    }
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, err
    }
    return &DB{Pool: pool}, nil
}

func (db *DB) Close() { db.Pool.Close() }

