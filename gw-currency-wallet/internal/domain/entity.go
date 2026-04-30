package domain

import (
	"errors"
	"net/mail"
	"strings"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyRUB Currency = "RUB"
	CurrencyEUR Currency = "EUR"
)

var SupportedCurrencies = []Currency{CurrencyUSD, CurrencyRUB, CurrencyEUR}

var (
	ErrInvalidUserID           = errors.New("invalid user id")
	ErrInvalidUsername         = errors.New("invalid username")
	ErrInvalidEmail            = errors.New("invalid email")
	ErrInvalidPassword         = errors.New("invalid password")
	ErrInvalidCredentials      = errors.New("invalid username or password")
	ErrInvalidCurrency         = errors.New("invalid currency")
	ErrInvalidAmount           = errors.New("amount must be positive")
	ErrInsufficientFunds       = errors.New("insufficient funds")
	ErrSameCurrency            = errors.New("currencies must be different")
	ErrExchangeRateUnavailable = errors.New("exchange rate unavailable")
	ErrConvertedAmountTooSmall = errors.New("converted amount is too small")
)

type RegisterUser struct {
	Username string
	Email    string
	Password string
}

type LoginUser struct {
	Username string
	Password string
}

type User struct {
	ID       int64
	Username string
	Email    string
}

type UserCredentials struct {
	User
	PasswordHash string
}

type ExchangeOperation struct {
	UserID       int64
	FromCurrency Currency
	ToCurrency   Currency
	AmountMinor  int64
}

func (u RegisterUser) Validate() error {
	if strings.TrimSpace(u.Username) == "" {
		return ErrInvalidUsername
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(u.Email)); err != nil {
		return ErrInvalidEmail
	}
	if len(u.Password) < 6 {
		return ErrInvalidPassword
	}
	return nil
}

func (u LoginUser) Validate() error {
	if strings.TrimSpace(u.Username) == "" {
		return ErrInvalidUsername
	}
	if u.Password == "" {
		return ErrInvalidPassword
	}
	return nil
}

func (o ExchangeOperation) Validate() error {
	if o.UserID <= 0 {
		return ErrInvalidUserID
	}
	if !o.FromCurrency.IsValid() || !o.ToCurrency.IsValid() {
		return ErrInvalidCurrency
	}
	if o.FromCurrency == o.ToCurrency {
		return ErrSameCurrency
	}
	if o.AmountMinor <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

func NormalizeCurrency(code string) (Currency, error) {
	currency := Currency(strings.ToUpper(strings.TrimSpace(code)))
	if !currency.IsValid() {
		return "", ErrInvalidCurrency
	}
	return currency, nil
}

func (c Currency) IsValid() bool {
	switch c {
	case CurrencyUSD, CurrencyRUB, CurrencyEUR:
		return true
	default:
		return false
	}
}
