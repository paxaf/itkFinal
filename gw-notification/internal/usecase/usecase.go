package usecase

import (
	"context"
	"fmt"

	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	"github.com/paxaf/itkFinal/gw-notification/internal/storages"
)

type HandleLargeOperationsResult struct {
	Received int
	Valid    int
	Invalid  int
	Accepted int
}

type LargeOperationHandler interface {
	HandleLargeOperation(ctx context.Context, event domain.LargeOperationEvent) (HandleLargeOperationsResult, error)
	HandleLargeOperations(ctx context.Context, events []domain.LargeOperationEvent) (HandleLargeOperationsResult, error)
}

type Service struct {
	storage storages.Storage
}

func New(storage storages.Storage) *Service {
	return &Service{
		storage: storage,
	}
}

func (s *Service) HandleLargeOperation(ctx context.Context, event domain.LargeOperationEvent) (HandleLargeOperationsResult, error) {
	return s.HandleLargeOperations(ctx, []domain.LargeOperationEvent{event})
}

func (s *Service) HandleLargeOperations(ctx context.Context, events []domain.LargeOperationEvent) (HandleLargeOperationsResult, error) {
	result := HandleLargeOperationsResult{
		Received: len(events),
	}

	validEvents := make([]domain.LargeOperationEvent, 0, len(events))

	for _, event := range events {
		if err := event.Validate(); err != nil {
			result.Invalid++
			continue
		}

		validEvents = append(validEvents, event)
	}
	result.Valid = len(validEvents)

	if len(validEvents) == 0 {
		return result, nil
	}

	if err := s.storage.SaveLargeOperations(ctx, validEvents); err != nil {
		return result, fmt.Errorf("save large operations: %w", err)
	}

	result.Accepted = len(validEvents)
	return result, nil
}
