package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/stretchr/testify/suite"
)

type HelpersSuite struct {
	suite.Suite
}

func TestHelpersSuite(t *testing.T) {
	suite.Run(t, new(HelpersSuite))
}

func (s *HelpersSuite) TestParseWalletKey() {
	userID, currency, err := parseWalletKey("42:usd")

	s.Require().NoError(err)
	s.Require().Equal(int64(42), userID)
	s.Require().Equal(domain.CurrencyUSD, currency)
}

func (s *HelpersSuite) TestParseWalletKeyErrors() {
	_, _, err := parseWalletKey("bad")
	s.Require().Error(err)

	_, _, err = parseWalletKey("0:USD")
	s.Require().Error(err)

	_, _, err = parseWalletKey("1:GBP")
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

func (s *HelpersSuite) TestPgErrorHelpers() {
	s.Require().True(isUniqueViolation(&pgconn.PgError{Code: "23505"}))
	s.Require().False(isUniqueViolation(&pgconn.PgError{Code: "23503"}))
	s.Require().True(isForeignKeyViolation(&pgconn.PgError{Code: "23503"}))
	s.Require().False(isForeignKeyViolation(&pgconn.PgError{Code: "23505"}))
}