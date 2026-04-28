package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"golang.org/x/crypto/bcrypt"
)

type TokenManager interface {
	Generate(userID int64) (string, error)
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
	WalletOperation(ctx context.Context, op domain.WalletOperation) error
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
}

func New(storage storages.Storage, tokenManager TokenManager, exchanger ExchangeProvider) *Service {
	return &Service{
		storage:      storage,
		tokenManager: tokenManager,
		exchanger:    exchanger,
	}
}

func (s *Service) Register(ctx context.Context, user domain.RegisterUser) (domain.User, error) {
	user.Username = strings.TrimSpace(user.Username)
	user.Email = strings.TrimSpace(user.Email)

	if err := user.Validate(); err != nil {
		return domain.User{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	created, err := s.storage.CreateUser(ctx, user.Username, user.Email, string(passwordHash))
	if err != nil {
		return domain.User{}, err
	}

	return created, nil
}

func (s *Service) Login(ctx context.Context, user domain.LoginUser) (string, error) {
	user.Username = strings.TrimSpace(user.Username)

	if err := user.Validate(); err != nil {
		return "", err
	}

	credentials, err := s.storage.GetUserCredentialsByUsername(ctx, user.Username)
	if err != nil {
		if errors.Is(err, storages.ErrUserNotFound) {
			return "", domain.ErrInvalidCredentials
		}
		return "", err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(user.Password)); err != nil {
		return "", domain.ErrInvalidCredentials
	}

	token, err := s.tokenManager.Generate(credentials.ID)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) GetBalance(ctx context.Context, userID int64) (map[string]int64, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidUserID
	}

	balances, err := s.storage.GetBalances(ctx, userID)
	if err != nil {
		return nil, err
	}

	return normalizeBalances(balances)
}

func (s *Service) Deposit(ctx context.Context, userID int64, currencyCode string, amountMinor int64) (map[string]int64, error) {
	currency, err := domain.NormalizeCurrency(currencyCode)
	if err != nil {
		return nil, err
	}

	op := domain.WalletOperation{
		UserID:        userID,
		Currency:      currency,
		OperationType: domain.OperationDeposit,
		AmountMinor:   amountMinor,
	}
	if err = op.Validate(); err != nil {
		return nil, err
	}

	balances, err := s.storage.Deposit(ctx, userID, currency, amountMinor)
	if err != nil {
		return nil, err
	}

	return normalizeBalances(balances)
}

func (s *Service) Withdraw(ctx context.Context, userID int64, currencyCode string, amountMinor int64) (map[string]int64, error) {
	currency, err := domain.NormalizeCurrency(currencyCode)
	if err != nil {
		return nil, err
	}

	op := domain.WalletOperation{
		UserID:        userID,
		Currency:      currency,
		OperationType: domain.OperationWithdraw,
		AmountMinor:   amountMinor,
	}
	if err = op.Validate(); err != nil {
		return nil, err
	}

	balances, err := s.storage.Withdraw(ctx, userID, currency, amountMinor)
	if err != nil {
		return nil, err
	}

	return normalizeBalances(balances)
}

func (s *Service) WalletOperation(ctx context.Context, op domain.WalletOperation) error {
	if err := op.Validate(); err != nil {
		return err
	}

	operationID := uuid.NewString()
	if err := s.storage.EnqueueOperation(ctx, operationID, op.UserID, op.Currency, op.OperationType, op.AmountMinor); err != nil {
		return err
	}

	return nil
}

func (s *Service) GetExchangeRates(ctx context.Context) (map[string]float64, error) {
	if s.exchanger == nil {
		return nil, domain.ErrExchangeRateUnavailable
	}

	rates, err := s.exchanger.GetRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("get exchange rates: %w", err)
	}

	normalized := make(map[string]float64, len(rates))
	for currency, rate := range rates {
		code, normalizeErr := domain.NormalizeCurrency(currency)
		if normalizeErr != nil {
			return nil, fmt.Errorf("invalid exchange currency %q: %w", currency, normalizeErr)
		}
		if !isValidRate(rate) {
			return nil, domain.ErrExchangeRateUnavailable
		}
		normalized[string(code)] = rate
	}

	return normalized, nil
}

func (s *Service) Exchange(ctx context.Context, op domain.ExchangeOperation) (ExchangeResult, error) {
	if err := op.Validate(); err != nil {
		return ExchangeResult{}, err
	}
	if s.exchanger == nil {
		return ExchangeResult{}, domain.ErrExchangeRateUnavailable
	}

	rate, err := s.exchanger.GetRate(ctx, string(op.FromCurrency), string(op.ToCurrency))
	if err != nil {
		return ExchangeResult{}, fmt.Errorf("get exchange rate: %w", err)
	}
	if !isValidRate(rate) {
		return ExchangeResult{}, domain.ErrExchangeRateUnavailable
	}

	toAmountMinor := convertMinor(op.AmountMinor, rate)
	if toAmountMinor <= 0 {
		return ExchangeResult{}, domain.ErrConvertedAmountTooSmall
	}

	balances, err := s.storage.Exchange(ctx, op.UserID, op.FromCurrency, op.ToCurrency, op.AmountMinor, toAmountMinor)
	if err != nil {
		return ExchangeResult{}, err
	}

	normalized, err := normalizeBalances(balances)
	if err != nil {
		return ExchangeResult{}, err
	}

	return ExchangeResult{
		ExchangedAmountMinor: toAmountMinor,
		NewBalance:           normalized,
	}, nil
}

func normalizeBalances(balances map[string]int64) (map[string]int64, error) {
	normalized := make(map[string]int64, len(domain.SupportedCurrencies))
	for _, currency := range domain.SupportedCurrencies {
		normalized[string(currency)] = 0
	}
	for currency, amountMinor := range balances {
		code, err := domain.NormalizeCurrency(currency)
		if err != nil {
			return nil, fmt.Errorf("invalid currency in storage %q: %w", currency, err)
		}
		normalized[string(code)] = amountMinor
	}

	return normalized, nil
}

func isValidRate(rate float64) bool {
	return rate > 0 && !math.IsInf(rate, 0) && !math.IsNaN(rate)
}

func convertMinor(amountMinor int64, rate float64) int64 {
	return int64(math.Round(float64(amountMinor) * rate))
}
