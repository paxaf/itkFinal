package postgres

import (
	"testing"

	"github.com/paxaf/itkFinal/gw-exchanger/internal/config"
	"github.com/stretchr/testify/suite"
)

type ConnectorSuite struct {
	suite.Suite
}

func TestConnectorSuite(t *testing.T) {
	suite.Run(t, new(ConnectorSuite))
}

func (s *ConnectorSuite) TestCloseEmptyPool() {
	s.Require().NoError((&PgPool{}).Close())
}

func (s *ConnectorSuite) TestNewReturnsParseConfigError() {
	_, err := New(&config.Postgres{
		Host: "localhost",
		Port: -1,
		User: "postgres",
		Name: "exchange",
	})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "parse postgres config")
}

func (s *ConnectorSuite) TestNewReturnsPingError() {
	_, err := New(&config.Postgres{
		Host:         "127.0.0.1",
		Port:         1,
		User:         "postgres",
		Password:     "postgres",
		Name:         "exchange",
		SSLMode:      "disable",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "ping db")
}
