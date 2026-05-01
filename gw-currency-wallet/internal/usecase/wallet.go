package usecase

import (
	"context"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/clients/events"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
)

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

func (s *Service) Deposit(ctx context.Context, userID int64, currencyCode string, amountMinor int64) (result map[string]int64, err error) {
	currency, err := domain.NormalizeCurrency(currencyCode)
	if err != nil {
		return nil, err
	}
	defer func() {
		go s.checkAndPublish(userID, events.OperationTypeDeposit, currency, amountMinor, err)
	}()

	if userID <= 0 {
		return nil, domain.ErrInvalidUserID
	}
	if amountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	balances, err := s.storage.Deposit(ctx, userID, currency, amountMinor)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeBalances(balances)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}

func (s *Service) Withdraw(ctx context.Context, userID int64, currencyCode string, amountMinor int64) (result map[string]int64, err error) {
	currency, err := domain.NormalizeCurrency(currencyCode)
	if err != nil {
		return nil, err
	}
	defer func() {
		go s.checkAndPublish(userID, events.OperationTypeWithdraw, currency, amountMinor, err)
	}()

	if userID <= 0 {
		return nil, domain.ErrInvalidUserID
	}
	if amountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	balances, err := s.storage.Withdraw(ctx, userID, currency, amountMinor)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeBalances(balances)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}
