package storages

import (
	"context"

	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
)

type Storage interface {
	SaveOperations(ctx context.Context, events []domain.OperationEvent) error
	SaveOperation(ctx context.Context, event domain.OperationEvent) error
	Close(ctx context.Context) error
}
