package usecase

import (
	"context"
	"fmt"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
)

type QueueProcessor interface {
	ListPendingWallets(ctx context.Context, limit int) ([]string, error)
	ProcessWallet(ctx context.Context, walletKey string) error
}

type ProcessorConfig struct {
	BatchSize int
}

type ProcessorService struct {
	storage   storages.WalletBatchProcessor
	reader    storages.PendingWalletsReader
	batchSize int
}

func NewProcessor(storage interface {
	storages.PendingWalletsReader
	storages.WalletBatchProcessor
}, cfg ProcessorConfig) *ProcessorService {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 128
	}

	return &ProcessorService{
		storage:   storage,
		reader:    storage,
		batchSize: cfg.BatchSize,
	}
}

func (s *ProcessorService) ListPendingWallets(ctx context.Context, limit int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.reader.ListPendingWallets(ctx, limit)
}

func (s *ProcessorService) ProcessWallet(ctx context.Context, walletKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := s.storage.ProcessWalletBatch(ctx, walletKey, s.batchSize)
	if err != nil {
		return fmt.Errorf("process wallet %s: %w", walletKey, err)
	}

	return nil
}
