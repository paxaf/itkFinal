package usecase

import (
	"context"
	"fmt"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/clients/events"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
)

func (s *Service) GetExchangeRates(ctx context.Context) (map[string]float64, error) {
	if s.exchanger == nil {
		return nil, domain.ErrExchangeRateUnavailable
	}

	rates, err := s.exchanger.GetRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("get exchange rates: %w", err)
	}

	normalized, err := normalizeRates(rates)
	if err != nil {
		return nil, err
	}

	s.setRatesCache(normalized)
	return copyRates(normalized), nil
}

func (s *Service) Exchange(ctx context.Context, op domain.ExchangeOperation) (result ExchangeResult, err error) {
	defer func() {
		s.runPublishAsync(func() {
			s.checkAndPublish(op.UserID, events.OperationTypeExchange, op.FromCurrency, op.AmountMinor, err)
		})
	}()

	if err := op.Validate(); err != nil {
		return ExchangeResult{}, err
	}
	if s.exchanger == nil {
		return ExchangeResult{}, domain.ErrExchangeRateUnavailable
	}

	rate, ok := s.cachedRate(op.FromCurrency, op.ToCurrency)
	if !ok {
		var err error
		rate, err = s.exchanger.GetRate(ctx, string(op.FromCurrency), string(op.ToCurrency))
		if err != nil {
			return ExchangeResult{}, fmt.Errorf("get exchange rate: %w", err)
		}
		if !isValidRate(rate) {
			return ExchangeResult{}, domain.ErrExchangeRateUnavailable
		}
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
