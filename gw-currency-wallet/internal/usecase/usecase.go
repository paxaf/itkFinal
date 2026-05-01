package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/clients/events"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
)

const (
	defaultRatesCacheTTL                   = 30 * time.Second
	defaultLargeOperationThresholdRubMinor = 3000000
)

type TokenManager interface {
	Generate(userID int64) (string, error)
}

type Logger interface {
	Error(message interface{}, args ...interface{})
}

type EventPublisher interface {
	PublishLargeOperation(ctx context.Context, event events.LargeOperationEvent) error
	PublishOperation(ctx context.Context, event events.OperationEvent) error
}

type ExchangeProvider interface {
	GetRates(ctx context.Context) (map[string]float64, error)
	GetRate(ctx context.Context, fromCurrency string, toCurrency string) (float64, error)
}

type Authenticator interface {
	Register(ctx context.Context, user domain.RegisterUser) (domain.User, error)
	Login(ctx context.Context, user domain.LoginUser) (string, error)
}

type WalletOperator interface {
	Deposit(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error)
	Withdraw(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error)
}

type BalanceGetter interface {
	GetBalance(ctx context.Context, userID int64) (map[string]int64, error)
}

type Exchanger interface {
	GetExchangeRates(ctx context.Context) (map[string]float64, error)
	Exchange(ctx context.Context, op domain.ExchangeOperation) (ExchangeResult, error)
}

type UseCase interface {
	Authenticator
	WalletOperator
	BalanceGetter
	Exchanger
}

type ExchangeResult struct {
	ExchangedAmountMinor int64
	NewBalance           map[string]int64
}

type Service struct {
	storage      storages.Storage
	tokenManager TokenManager
	exchanger    ExchangeProvider
	publisher    EventPublisher
	log          Logger

	largeOperationThresholdRubMinor int64

	ratesCacheMu  sync.RWMutex
	ratesCache    map[string]float64
	ratesCachedAt time.Time
	ratesCacheTTL time.Duration
}

func New(
	storage storages.Storage,
	tokenManager TokenManager,
	exchanger ExchangeProvider,
	publisher EventPublisher,
	largeOperationThresholdRubMinor int64,
	log Logger,
) *Service {
	if largeOperationThresholdRubMinor <= 0 {
		largeOperationThresholdRubMinor = defaultLargeOperationThresholdRubMinor
	}

	return &Service{
		storage:                         storage,
		tokenManager:                    tokenManager,
		exchanger:                       exchanger,
		publisher:                       publisher,
		log:                             log,
		largeOperationThresholdRubMinor: largeOperationThresholdRubMinor,
		ratesCacheTTL:                   defaultRatesCacheTTL,
	}
}
