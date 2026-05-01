package usecase

import (
	"fmt"
	"math"
	"time"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
)

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

func normalizeRates(rates map[string]float64) (map[string]float64, error) {
	normalized := make(map[string]float64, len(domain.SupportedCurrencies))
	for currency, rate := range rates {
		code, err := domain.NormalizeCurrency(currency)
		if err != nil {
			return nil, fmt.Errorf("invalid exchange currency %q: %w", currency, err)
		}
		if !isValidRate(rate) {
			return nil, domain.ErrExchangeRateUnavailable
		}
		normalized[string(code)] = rate
	}

	for _, currency := range domain.SupportedCurrencies {
		if !isValidRate(normalized[string(currency)]) {
			return nil, domain.ErrExchangeRateUnavailable
		}
	}

	return normalized, nil
}

func (s *Service) setRatesCache(rates map[string]float64) {
	s.ratesCacheMu.Lock()
	defer s.ratesCacheMu.Unlock()

	s.ratesCache = copyRates(rates)
	s.ratesCachedAt = time.Now()
}

func (s *Service) cachedRate(fromCurrency domain.Currency, toCurrency domain.Currency) (float64, bool) {
	s.ratesCacheMu.RLock()
	defer s.ratesCacheMu.RUnlock()

	if len(s.ratesCache) == 0 || s.ratesCacheTTL <= 0 || time.Since(s.ratesCachedAt) > s.ratesCacheTTL {
		return 0, false
	}

	fromRate := s.ratesCache[string(fromCurrency)]
	toRate := s.ratesCache[string(toCurrency)]
	if !isValidRate(fromRate) || !isValidRate(toRate) {
		return 0, false
	}

	return toRate / fromRate, true
}

func copyRates(rates map[string]float64) map[string]float64 {
	copied := make(map[string]float64, len(rates))
	for currency, rate := range rates {
		copied[currency] = rate
	}
	return copied
}

func isValidRate(rate float64) bool {
	return rate > 0 && !math.IsInf(rate, 0) && !math.IsNaN(rate)
}

func convertMinor(amountMinor int64, rate float64) int64 {
	return int64(math.Round(float64(amountMinor) * rate))
}
