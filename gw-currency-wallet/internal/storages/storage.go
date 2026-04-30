package storages

import (
	"context"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
)

type UserCreator interface {
	CreateUser(ctx context.Context, username string, email string, passwordHash string) (domain.User, error)
}

type UserReader interface {
	GetUserCredentialsByUsername(ctx context.Context, username string) (domain.UserCredentials, error)
}

type BalanceReader interface {
	GetBalances(ctx context.Context, userID int64) (map[string]int64, error)
}

type BalanceChanger interface {
	Deposit(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error)
	Withdraw(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error)
	Exchange(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error)
}

type Storage interface {
	UserCreator
	UserReader
	BalanceReader
	BalanceChanger
	Close() error
}
