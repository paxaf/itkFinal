package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/paxaf/itkFinal/gw-exchanger/internal/storages"
)

var ErrInvalidCurrency = errors.New("invalid currency code")

var supportedCurrencyCodes = []string{"USD", "RUB", "EUR"}

var supportedCurrencies = map[string]struct{}{
	"USD": {},
	"RUB": {},
	"EUR": {},
}

type Exchanger interface {
	GetRate(ctx context.Context, from, to string) (float64, error)
	GetRates(ctx context.Context) (map[string]float64, error)
}

type Service struct {
	storage storages.Storage
}

func New(storage storages.Storage) *Service {
	return &Service{storage: storage}
}

func (s *Service) GetRate(ctx context.Context, from, to string) (float64, error) {
	fromCurrency, err := normalizeCurrency(from)
	if err != nil {
		return 0, fmt.Errorf("from currency: %w", err)
	}

	toCurrency, err := normalizeCurrency(to)
	if err != nil {
		return 0, fmt.Errorf("to currency: %w", err)
	}

	if fromCurrency == toCurrency {
		return 1, nil
	}

	rate, err := s.storage.GetRate(ctx, fromCurrency, toCurrency)
	if err != nil {
		return 0, fmt.Errorf("get rate from storage: %w", err)
	}

	return rate, nil
}

func (s *Service) GetRates(ctx context.Context) (map[string]float64, error) {
	rates, err := s.storage.GetRates(ctx)
	if err != nil {
		return nil, err
	}

	return normalizeRates(rates)
}

func normalizeCurrency(currency string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(currency))
	if len(code) != 3 {
		return "", ErrInvalidCurrency
	}
	if _, ok := supportedCurrencies[code]; !ok {
		return "", ErrInvalidCurrency
	}

	return code, nil
}

func normalizeRates(rates map[string]float64) (map[string]float64, error) {
	normalized := make(map[string]float64, len(supportedCurrencyCodes))
	for currency, rate := range rates {
		code, err := normalizeCurrency(currency)
		if err != nil {
			return nil, fmt.Errorf("currency %q: %w", currency, err)
		}
		normalized[code] = rate
	}

	for _, currency := range supportedCurrencyCodes {
		if _, ok := normalized[currency]; !ok {
			return nil, fmt.Errorf("currency %q: %w", currency, ErrInvalidCurrency)
		}
	}

	return normalized, nil
}
