package postgres

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/config"
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

func (s *PostgresIntegrationSuite) SetupTest() {
	_, err := s.db.ExecContext(s.ctx, `
TRUNCATE exchange_rates;
INSERT INTO exchange_rates (currency_code, units_per_usd)
VALUES
	('USD', 1.0000),
	('RUB', 90.0000),
	('EUR', 0.9200);
`)
	s.Require().NoError(err)
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

func (s *PostgresIntegrationSuite) TestNewConnector() {
	pool, err := New(s.postgresConfig())

	s.Require().NoError(err)
	s.Require().NoError(pool.Close())
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

func (s *PostgresIntegrationSuite) TestGetRateForSameCurrency() {
	rate, err := s.pool.GetRate(s.ctx, "USD", "USD")

	s.Require().NoError(err)
	s.Require().Equal(1.0, rate)
}

func (s *PostgresIntegrationSuite) TestGetRateReturnsErrorWhenCurrencyMissing() {
	_, err := s.pool.GetRate(s.ctx, "USD", "GBP")

	s.Require().Error(err)
}

func (s *PostgresIntegrationSuite) TestGetRateReturnsErrorWhenBothCurrenciesMissing() {
	_, err := s.pool.GetRate(s.ctx, "GBP", "JPY")

	s.Require().Error(err)
}

func (s *PostgresIntegrationSuite) TestGetRatesReturnsEmptyMap() {
	_, err := s.db.ExecContext(s.ctx, "TRUNCATE exchange_rates")
	s.Require().NoError(err)

	rates, err := s.pool.GetRates(s.ctx)

	s.Require().NoError(err)
	s.Require().Empty(rates)
}

func (s *PostgresIntegrationSuite) TestClosedPoolReturnsErrors() {
	pool, err := New(s.postgresConfig())
	s.Require().NoError(err)
	s.Require().NoError(pool.Close())

	_, err = pool.GetRate(s.ctx, "USD", "RUB")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "query get rate")

	_, err = pool.GetRates(s.ctx)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "query get rates")
}

func (s *PostgresIntegrationSuite) postgresConfig() *config.Postgres {
	host, err := s.container.Host(s.ctx)
	s.Require().NoError(err)

	port, err := s.container.MappedPort(s.ctx, "5432/tcp")
	s.Require().NoError(err)

	portNumber, err := strconv.Atoi(port.Port())
	s.Require().NoError(err)

	return &config.Postgres{
		Host:         host,
		Port:         portNumber,
		User:         "postgres",
		Password:     "postgres",
		Name:         "exchange",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 1,
	}
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
