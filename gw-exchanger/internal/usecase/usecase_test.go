package usecase

import (
	"context"
	"errors"
	"testing"

	storagesmocks "github.com/paxaf/itkFinal/gw-exchanger/internal/mocks/storages"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServiceGetRateSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage := storagesmocks.NewStorageMock(t)
	storage.EXPECT().GetRate(mock.Anything, "USD", "RUB").Return(90.0, nil).Once()

	svc := New(storage)

	rate, err := svc.GetRate(ctx, "usd", "rub")
	require.NoError(t, err)
	require.Equal(t, 90.0, rate)
}

func TestServiceGetRateSameCurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage := storagesmocks.NewStorageMock(t)
	svc := New(storage)

	rate, err := svc.GetRate(ctx, "usd", "usd")
	require.NoError(t, err)
	require.Equal(t, 1.0, rate)
}

func TestServiceGetRateInvalidFromCurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage := storagesmocks.NewStorageMock(t)
	svc := New(storage)

	_, err := svc.GetRate(ctx, "us", "RUB")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCurrency)
}

func TestServiceGetRateInvalidToCurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage := storagesmocks.NewStorageMock(t)
	svc := New(storage)

	_, err := svc.GetRate(ctx, "USD", "r")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCurrency)
}

func TestServiceGetRateStorageError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageErr := errors.New("db error")
	storage := storagesmocks.NewStorageMock(t)
	storage.EXPECT().GetRate(mock.Anything, "USD", "EUR").Return(0.0, storageErr).Once()

	svc := New(storage)

	_, err := svc.GetRate(ctx, "USD", "EUR")
	require.Error(t, err)
	require.ErrorIs(t, err, storageErr)
}

func TestServiceGetRatesSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expected := map[string]float64{
		"USD": 1,
		"RUB": 90,
		"EUR": 0.92,
	}

	storage := storagesmocks.NewStorageMock(t)
	storage.EXPECT().GetRates(mock.Anything).Return(expected, nil).Once()

	svc := New(storage)

	rates, err := svc.GetRates(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, rates)
}

func TestServiceGetRatesStorageError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storageErr := errors.New("db error")

	storage := storagesmocks.NewStorageMock(t)
	storage.EXPECT().GetRates(mock.Anything).Return(nil, storageErr).Once()

	svc := New(storage)

	_, err := svc.GetRates(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, storageErr)
}
