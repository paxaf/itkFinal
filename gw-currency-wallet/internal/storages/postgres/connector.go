package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/config"
)

const connectTimeout = 5 * time.Second

type PgPool struct {
	pool *pgxpool.Pool
}

func New(cfg *config.Postgres) (*PgPool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PgPool{pool: pool}, nil
}

func (p *PgPool) Close() error {
	if p == nil || p.pool == nil {
		return nil
	}

	p.pool.Close()
	return nil
}
