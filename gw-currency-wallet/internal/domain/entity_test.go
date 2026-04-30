package domain

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type DomainSuite struct {
	suite.Suite
}

func TestDomainSuite(t *testing.T) {
	suite.Run(t, new(DomainSuite))
}

func (s *DomainSuite) TestRegisterUserValidate() {
	s.Require().NoError(RegisterUser{Username: "paxaf", Email: "paxaf@example.com", Password: "secret1"}.Validate())
	s.Require().ErrorIs(RegisterUser{Email: "paxaf@example.com", Password: "secret1"}.Validate(), ErrInvalidUsername)
	s.Require().ErrorIs(RegisterUser{Username: "paxaf", Email: "bad", Password: "secret1"}.Validate(), ErrInvalidEmail)
	s.Require().ErrorIs(RegisterUser{Username: "paxaf", Email: "paxaf@example.com", Password: "123"}.Validate(), ErrInvalidPassword)
}

func (s *DomainSuite) TestLoginUserValidate() {
	s.Require().NoError(LoginUser{Username: "paxaf", Password: "secret1"}.Validate())
	s.Require().ErrorIs(LoginUser{Password: "secret1"}.Validate(), ErrInvalidUsername)
	s.Require().ErrorIs(LoginUser{Username: "paxaf"}.Validate(), ErrInvalidPassword)
}

func (s *DomainSuite) TestExchangeOperationValidate() {
	op := ExchangeOperation{UserID: 1, FromCurrency: CurrencyUSD, ToCurrency: CurrencyEUR, AmountMinor: 100}
	s.Require().NoError(op.Validate())

	op.UserID = 0
	s.Require().ErrorIs(op.Validate(), ErrInvalidUserID)
	op.UserID = 1
	op.FromCurrency = Currency("GBP")
	s.Require().ErrorIs(op.Validate(), ErrInvalidCurrency)
	op.FromCurrency = CurrencyUSD
	op.ToCurrency = CurrencyUSD
	s.Require().ErrorIs(op.Validate(), ErrSameCurrency)
	op.ToCurrency = CurrencyEUR
	op.AmountMinor = 0
	s.Require().ErrorIs(op.Validate(), ErrInvalidAmount)
}

func (s *DomainSuite) TestNormalizeCurrency() {
	currency, err := NormalizeCurrency(" usd ")
	s.Require().NoError(err)
	s.Require().Equal(CurrencyUSD, currency)

	_, err = NormalizeCurrency("gbp")
	s.Require().ErrorIs(err, ErrInvalidCurrency)
}

func (s *DomainSuite) TestCurrencyValid() {
	s.Require().True(CurrencyEUR.IsValid())
	s.Require().False(Currency("GBP").IsValid())
}
