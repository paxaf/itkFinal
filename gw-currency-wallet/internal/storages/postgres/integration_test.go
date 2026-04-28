package postgres

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
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
		tcpostgres.WithDatabase("wallet"),
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

func (s *PostgresIntegrationSuite) SetupTest() {
	_, err := s.db.ExecContext(s.ctx, `TRUNCATE TABLE wallet_operations, balances, users RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
}

func (s *PostgresIntegrationSuite) TestWalletFlow() {
	user, err := s.pool.CreateUser(s.ctx, "paxaf", "paxaf@example.com", "hash")
	s.Require().NoError(err)
	s.Require().NotZero(user.ID)

	balance, err := s.pool.GetBalances(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(int64(0), balance["USD"])
	s.Require().Equal(int64(0), balance["EUR"])
	s.Require().Equal(int64(0), balance["RUB"])

	balance, err = s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 10000)
	s.Require().NoError(err)
	s.Require().Equal(int64(10000), balance["USD"])

	balance, err = s.pool.Withdraw(s.ctx, user.ID, domain.CurrencyUSD, 2500)
	s.Require().NoError(err)
	s.Require().Equal(int64(7500), balance["USD"])

	_, err = s.pool.Withdraw(s.ctx, user.ID, domain.CurrencyUSD, 10000)
	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)

	balance, err = s.pool.Exchange(s.ctx, user.ID, domain.CurrencyUSD, domain.CurrencyEUR, 5000, 4600)
	s.Require().NoError(err)
	s.Require().Equal(int64(2500), balance["USD"])
	s.Require().Equal(int64(4600), balance["EUR"])
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
