package postgres

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var skipIntegration = flag.Bool("skip-integration", false, "skip integration tests")

type PostgresIntegrationSuite struct {
	suite.Suite

	ctx       context.Context
	container *tcpostgres.PostgresContainer
	db        *sql.DB
	pool      *PgPool
}

func TestPostgresIntegrationSuite(t *testing.T) {
	if *skipIntegration {
		t.Skip("integration tests are skipped by -skip-integration")
	}
	if testing.Short() {
		t.Skip("integration tests are skipped in short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("integration tests are skipped: docker executable not found")
	}
	tc.SkipIfProviderIsNotHealthy(t)

	suite.Run(t, new(PostgresIntegrationSuite))
}

func (s *PostgresIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := tcpostgres.Run(
		s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("exchange"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
	)
	s.Require().NoError(err)
	s.container = container

	dsn, err := container.ConnectionString(s.ctx, "sslmode=disable")
	s.Require().NoError(err)

	db, err := sql.Open("pgx", dsn)
	s.Require().NoError(err)
	s.db = db

	s.Require().NoError(waitForSQLReady(s.ctx, s.db, 30*time.Second))
	s.Require().NoError(goose.SetDialect("postgres"))
	s.Require().NoError(goose.Up(s.db, "../../../migrations"))

	poolCfg, err := pgxpool.ParseConfig(dsn)
	s.Require().NoError(err)

	pgxPool, err := pgxpool.NewWithConfig(s.ctx, poolCfg)
	s.Require().NoError(err)
	s.Require().NoError(waitForPoolReady(s.ctx, pgxPool, 30*time.Second))

	s.pool = &PgPool{pool: pgxPool}
}

func (s *PostgresIntegrationSuite) TearDownSuite() {
	if s.pool != nil {
		_ = s.pool.Close()
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *PostgresIntegrationSuite) TestGetRates() {
	rates, err := s.pool.GetRates(s.ctx)

	s.Require().NoError(err)
	s.Require().Equal(1.0, rates["USD"])
	s.Require().Equal(90.0, rates["RUB"])
	s.Require().Equal(0.92, rates["EUR"])
}

func (s *PostgresIntegrationSuite) TestGetRate() {
	rate, err := s.pool.GetRate(s.ctx, "USD", "RUB")

	s.Require().NoError(err)
	s.Require().Equal(90.0, rate)

	rate, err = s.pool.GetRate(s.ctx, "RUB", "EUR")

	s.Require().NoError(err)
	s.Require().True(math.Abs(rate-(0.92/90.0)) < 0.000001)
}

func (s *PostgresIntegrationSuite) TestGetRateReturnsErrorWhenCurrencyMissing() {
	_, err := s.pool.GetRate(s.ctx, "USD", "GBP")

	s.Require().Error(err)
}

func waitForSQLReady(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		pingCtx, pingCancel := context.WithTimeout(deadlineCtx, 2*time.Second)
		err := db.PingContext(pingCtx)
		pingCancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("postgres sql is not ready: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func waitForPoolReady(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		pingCtx, pingCancel := context.WithTimeout(deadlineCtx, 2*time.Second)
		err := pool.Ping(pingCtx)
		pingCancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("postgres pool is not ready: %w", lastErr)
		case <-ticker.C:
		}
	}
}
