package postgres

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/config"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
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
	_, err := s.db.ExecContext(s.ctx, `TRUNCATE TABLE balances, users RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
}

func (s *PostgresIntegrationSuite) TestWalletFlow() {
	user := s.createUser("paxaf")

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

func (s *PostgresIntegrationSuite) TestNewConnector() {
	host, err := s.container.Host(s.ctx)
	s.Require().NoError(err)

	port, err := s.container.MappedPort(s.ctx, "5432/tcp")
	s.Require().NoError(err)

	portNumber, err := strconv.Atoi(port.Port())
	s.Require().NoError(err)

	pool, err := New(&config.Postgres{
		Host:         host,
		Port:         portNumber,
		User:         "postgres",
		Password:     "postgres",
		Name:         "wallet",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 1,
	})
	s.Require().NoError(err)
	s.Require().NoError(pool.Close())
}

func (s *PostgresIntegrationSuite) TestCreateUserCreatesBalances() {
	user := s.createUser("paxaf")

	balances, err := s.pool.GetBalances(s.ctx, user.ID)

	s.Require().NoError(err)
	s.Require().Equal(map[string]int64{
		"EUR": 0,
		"RUB": 0,
		"USD": 0,
	}, balances)
}

func (s *PostgresIntegrationSuite) TestCreateUserDuplicateReturnsErrorAndRollsBack() {
	_ = s.createUser("paxaf")

	_, err := s.pool.CreateUser(s.ctx, "paxaf", "other@example.com", "hash")
	s.Require().ErrorIs(err, storages.ErrDuplicateUser)

	user := s.createUser("next")
	_, err = s.pool.Deposit(s.ctx, user.ID, domain.CurrencyRUB, 1000)
	s.Require().NoError(err)
}

func (s *PostgresIntegrationSuite) TestCreateUserDuplicateEmailReturnsErrorAndRollsBack() {
	_ = s.createUser("paxaf")

	_, err := s.pool.CreateUser(s.ctx, "other", "paxaf@example.com", "hash")
	s.Require().ErrorIs(err, storages.ErrDuplicateUser)

	user := s.createUser("next")
	_, err = s.pool.Deposit(s.ctx, user.ID, domain.CurrencyRUB, 1000)
	s.Require().NoError(err)
}

func (s *PostgresIntegrationSuite) TestCreateUserBalanceInsertErrorRollsBack() {
	cleanup := s.installFailingBalanceInsertTrigger()
	defer cleanup()

	_, err := s.pool.CreateUser(s.ctx, "paxaf", "paxaf@example.com", "hash")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "insert balance")

	_, err = s.pool.GetUserCredentialsByUsername(s.ctx, "paxaf")
	s.Require().ErrorIs(err, storages.ErrUserNotFound)
}

func (s *PostgresIntegrationSuite) TestGetUserCredentialsByUsername() {
	user := s.createUser("paxaf")

	credentials, err := s.pool.GetUserCredentialsByUsername(s.ctx, "paxaf")

	s.Require().NoError(err)
	s.Require().Equal(user.ID, credentials.ID)
	s.Require().Equal(user.Username, credentials.Username)
	s.Require().Equal(user.Email, credentials.Email)
	s.Require().Equal("hash", credentials.PasswordHash)
}

func (s *PostgresIntegrationSuite) TestGetUserCredentialsByUsernameNotFound() {
	_, err := s.pool.GetUserCredentialsByUsername(s.ctx, "missing")

	s.Require().ErrorIs(err, storages.ErrUserNotFound)
}

func (s *PostgresIntegrationSuite) TestGetBalancesUserNotFound() {
	_, err := s.pool.GetBalances(s.ctx, 999)

	s.Require().ErrorIs(err, storages.ErrUserNotFound)
}

func (s *PostgresIntegrationSuite) TestDepositErrors() {
	user := s.createUser("paxaf")

	_, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 0)
	s.Require().ErrorIs(err, domain.ErrInvalidAmount)

	_, err = s.pool.Deposit(s.ctx, 999, domain.CurrencyUSD, 1000)
	s.Require().ErrorIs(err, storages.ErrUserNotFound)

	balances, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().NoError(err)
	s.Require().Equal(int64(1000), balances["USD"])
}

func (s *PostgresIntegrationSuite) TestDepositUserNotFoundRollsBack() {
	_, err := s.pool.Deposit(s.ctx, 999, domain.CurrencyUSD, 1000)
	s.Require().ErrorIs(err, storages.ErrUserNotFound)

	user := s.createUser("paxaf")
	balances, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().NoError(err)
	s.Require().Equal(int64(1000), balances["USD"])
}

func (s *PostgresIntegrationSuite) TestWithdrawErrors() {
	user := s.createUser("paxaf")

	_, err := s.pool.Withdraw(s.ctx, user.ID, domain.CurrencyUSD, 0)
	s.Require().ErrorIs(err, domain.ErrInvalidAmount)

	_, err = s.pool.Withdraw(s.ctx, 999, domain.CurrencyUSD, 1000)
	s.Require().ErrorIs(err, storages.ErrUserNotFound)

	_, err = s.pool.Withdraw(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)

	balances, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().NoError(err)
	s.Require().Equal(int64(1000), balances["USD"])
}

func (s *PostgresIntegrationSuite) TestWithdrawUserNotFoundRollsBack() {
	_, err := s.pool.Withdraw(s.ctx, 999, domain.CurrencyUSD, 1000)
	s.Require().ErrorIs(err, storages.ErrUserNotFound)

	user := s.createUser("paxaf")
	balances, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().NoError(err)
	s.Require().Equal(int64(1000), balances["USD"])
}

func (s *PostgresIntegrationSuite) TestExchangeErrors() {
	user := s.createUser("paxaf")

	_, err := s.pool.Exchange(s.ctx, user.ID, domain.CurrencyUSD, domain.CurrencyEUR, 0, 1000)
	s.Require().ErrorIs(err, domain.ErrInvalidAmount)

	_, err = s.pool.Exchange(s.ctx, user.ID, domain.CurrencyUSD, domain.CurrencyUSD, 1000, 1000)
	s.Require().ErrorIs(err, domain.ErrSameCurrency)

	_, err = s.pool.Exchange(s.ctx, 999, domain.CurrencyUSD, domain.CurrencyEUR, 1000, 1000)
	s.Require().ErrorIs(err, storages.ErrUserNotFound)

	_, err = s.pool.Exchange(s.ctx, user.ID, domain.CurrencyUSD, domain.CurrencyEUR, 1000, 900)
	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)

	balances, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().NoError(err)
	s.Require().Equal(int64(1000), balances["USD"])
}

func (s *PostgresIntegrationSuite) TestExchangeUserNotFoundRollsBack() {
	_, err := s.pool.Exchange(s.ctx, 999, domain.CurrencyUSD, domain.CurrencyEUR, 1000, 900)
	s.Require().ErrorIs(err, storages.ErrUserNotFound)

	user := s.createUser("paxaf")
	balances, err := s.pool.Deposit(s.ctx, user.ID, domain.CurrencyUSD, 1000)
	s.Require().NoError(err)
	s.Require().Equal(int64(1000), balances["USD"])
}

func (s *PostgresIntegrationSuite) TestCanceledContextErrors() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	_, err := s.pool.CreateUser(ctx, "paxaf", "paxaf@example.com", "hash")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "begin tx")

	_, err = s.pool.GetUserCredentialsByUsername(ctx, "paxaf")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "get user credentials")

	_, err = s.pool.GetBalances(ctx, 1)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "get balances")

	_, err = s.pool.Deposit(ctx, 1, domain.CurrencyUSD, 1000)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "begin tx")

	_, err = s.pool.Withdraw(ctx, 1, domain.CurrencyUSD, 1000)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "begin tx")

	_, err = s.pool.Exchange(ctx, 1, domain.CurrencyUSD, domain.CurrencyEUR, 1000, 900)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "begin tx")
}

func (s *PostgresIntegrationSuite) TestTransactionHelpersErrors() {
	user := s.createUser("paxaf")

	err := s.withTx(func(tx pgx.Tx) {
		_, helperErr := lockBalance(s.ctx, tx, 999, domain.CurrencyUSD)
		s.Require().ErrorIs(helperErr, storages.ErrUserNotFound)
	})
	s.Require().NoError(err)

	err = s.withTx(func(tx pgx.Tx) {
		helperErr := updateBalanceDelta(s.ctx, tx, 999, domain.CurrencyUSD, 1000)
		s.Require().ErrorIs(helperErr, storages.ErrUserNotFound)
	})
	s.Require().NoError(err)

	_, err = s.db.ExecContext(s.ctx, "DELETE FROM balances WHERE user_id = $1 AND currency_code = $2", user.ID, domain.CurrencyEUR)
	s.Require().NoError(err)

	err = s.withTx(func(tx pgx.Tx) {
		_, helperErr := lockExchangeBalances(s.ctx, tx, user.ID, domain.CurrencyUSD, domain.CurrencyEUR)
		s.Require().ErrorIs(helperErr, storages.ErrUserNotFound)
	})
	s.Require().NoError(err)

	_, err = s.db.ExecContext(s.ctx, "DELETE FROM balances WHERE user_id = $1", user.ID)
	s.Require().NoError(err)

	err = s.withTx(func(tx pgx.Tx) {
		_, helperErr := getBalancesTx(s.ctx, tx, user.ID)
		s.Require().ErrorIs(helperErr, storages.ErrUserNotFound)
	})
	s.Require().NoError(err)
}

func (s *PostgresIntegrationSuite) TestRollbackPanicsAndRollsBack() {
	tx, err := s.pool.pool.Begin(s.ctx)
	s.Require().NoError(err)

	s.Require().PanicsWithValue("boom", func() {
		var opErr error
		defer rollback(tx, &opErr)
		panic("boom")
	})
}

func (s *PostgresIntegrationSuite) createUser(username string) domain.User {
	user, err := s.pool.CreateUser(s.ctx, username, username+"@example.com", "hash")
	s.Require().NoError(err)
	s.Require().NotZero(user.ID)
	return user
}

func (s *PostgresIntegrationSuite) withTx(fn func(pgx.Tx)) error {
	tx, err := s.pool.pool.Begin(s.ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	fn(tx)
	return nil
}

func (s *PostgresIntegrationSuite) installFailingBalanceInsertTrigger() func() {
	_, err := s.db.ExecContext(s.ctx, `
CREATE OR REPLACE FUNCTION fail_balance_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
	RAISE EXCEPTION 'balance insert failed';
END;
$$;

CREATE TRIGGER fail_balance_insert
BEFORE INSERT ON balances
FOR EACH ROW
EXECUTE FUNCTION fail_balance_insert();
`)
	s.Require().NoError(err)

	return func() {
		_, cleanupErr := s.db.ExecContext(context.Background(), `
DROP TRIGGER IF EXISTS fail_balance_insert ON balances;
DROP FUNCTION IF EXISTS fail_balance_insert();
`)
		s.Require().NoError(cleanupErr)
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
