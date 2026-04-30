package storages

import (
	"context"

	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
)

type Storage interface {
	SaveLargeOperations(ctx context.Context, events []domain.LargeOperationEvent) error
	SaveLargeOperation(ctx context.Context, event domain.LargeOperationEvent) error
	Close(ctx context.Context) error
}
