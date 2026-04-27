package storages

import "context"

type Storage interface {
	GetRate(ctx context.Context, from, to string) (float64, error)
	GetRates(ctx context.Context) (map[string]float64, error)
	Close() error
}
