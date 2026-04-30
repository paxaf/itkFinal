package postgres

import (
	"context"
	"fmt"
)

const (
	queryGetRate string = `SELECT
  MAX(CASE WHEN currency_code = $1 THEN units_per_usd END) AS from_units,
  MAX(CASE WHEN currency_code = $2 THEN units_per_usd END) AS to_units
FROM exchange_rates
WHERE currency_code IN ($1, $2);`
	queryGetRates string = `SELECT currency_code, units_per_usd FROM exchange_rates`
)

func (s *PgPool) GetRate(ctx context.Context, from, to string) (float64, error) {
	var fromUnits, toUnits *float64
	err := s.pool.QueryRow(ctx, queryGetRate, from, to).Scan(&fromUnits, &toUnits)
	if err != nil {
		return 0, fmt.Errorf("query get rate: %w", err)
	}
	if fromUnits == nil || toUnits == nil {
		return 0, fmt.Errorf("rate not found for currencies %s and %s", from, to)
	}
	return *toUnits / *fromUnits, nil
}

func (s *PgPool) GetRates(ctx context.Context) (map[string]float64, error) {
	rows, err := s.pool.Query(ctx, queryGetRates)
	if err != nil {
		return nil, fmt.Errorf("query get rates: %w", err)
	}
	defer rows.Close()

	rates := make(map[string]float64)
	for rows.Next() {
		var currencyCode string
		var unitsPerUSD float64
		if err := rows.Scan(&currencyCode, &unitsPerUSD); err != nil {
			return nil, fmt.Errorf("scan rate: %w", err)
		}
		rates[currencyCode] = unitsPerUSD
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rates: %w", err)
	}
	return rates, nil
}
