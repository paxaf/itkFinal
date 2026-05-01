package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/config"
	"github.com/stretchr/testify/suite"
)

type HelpersSuite struct {
	suite.Suite
}

func TestHelpersSuite(t *testing.T) {
	suite.Run(t, new(HelpersSuite))
}

func (s *HelpersSuite) TestPgErrorHelpers() {
	s.Require().True(isUniqueViolation(&pgconn.PgError{Code: "23505"}))
	s.Require().False(isUniqueViolation(&pgconn.PgError{Code: "23503"}))
}

func (s *HelpersSuite) TestCloseNilPool() {
	var pool *PgPool
	s.Require().NoError(pool.Close())
	s.Require().NoError((&PgPool{}).Close())
}

func (s *HelpersSuite) TestNewReturnsParseConfigError() {
	_, err := New(&config.Postgres{
		Host: "localhost",
		Port: -1,
		User: "postgres",
		Name: "wallet",
	})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "parse postgres config")
}

func (s *HelpersSuite) TestNewReturnsPingError() {
	_, err := New(&config.Postgres{
		Host:         "127.0.0.1",
		Port:         1,
		User:         "postgres",
		Password:     "postgres",
		Name:         "wallet",
		SSLMode:      "disable",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "ping postgres")
}
