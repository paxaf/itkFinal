package usecase

import (
	"context"
	"fmt"

	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
	"github.com/paxaf/itkFinal/gw-analytics/internal/storages"
)

type HandleOperationsResult struct {
	Received int
	Valid    int
	Invalid  int
	Accepted int
}

type OperationHandler interface {
	HandleOperation(ctx context.Context, event domain.OperationEvent) (HandleOperationsResult, error)
	HandleOperations(ctx context.Context, events []domain.OperationEvent) (HandleOperationsResult, error)
}

type Service struct {
	storage storages.Storage
}

func New(storage storages.Storage) *Service {
	return &Service{
		storage: storage,
	}
}

func (s *Service) HandleOperation(ctx context.Context, event domain.OperationEvent) (HandleOperationsResult, error) {
	return s.HandleOperations(ctx, []domain.OperationEvent{event})
}

func (s *Service) HandleOperations(ctx context.Context, events []domain.OperationEvent) (HandleOperationsResult, error) {
	result := HandleOperationsResult{
		Received: len(events),
	}

	validEvents := make([]domain.OperationEvent, 0, len(events))

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

	if err := s.storage.SaveOperations(ctx, validEvents); err != nil {
		return result, fmt.Errorf("save operations: %w", err)
	}

	result.Accepted = len(validEvents)
	return result, nil
}
