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

type OperationEnqueuer interface {
	EnqueueOperation(ctx context.Context, operationID string, userID int64, currency domain.Currency, operationType domain.OperationType, amountMinor int64) error
}

type PendingWalletsReader interface {
	ListPendingWallets(ctx context.Context, limit int) ([]string, error)
}

type WalletBatchProcessor interface {
	ProcessWalletBatch(ctx context.Context, walletKey string, batchSize int) (int, error)
}

type Storage interface {
	UserCreator
	UserReader
	BalanceReader
	BalanceChanger
	OperationEnqueuer
	PendingWalletsReader
	WalletBatchProcessor
	Close() error
}
