package databaseengine

import (
	"context"
	"time"

	logger "raven/auth/Logging"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewDB(ctx context.Context, connString string) (*DB, error) {
	config, err := pgxpool.ParseConfig(connString)

	if err != nil {
		logger.Log("Unable to parse the PG config", logger.Error)
	}

	config.MaxConns = 50
	config.MaxConnLifetime = 10 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}
