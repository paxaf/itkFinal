package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/clients/events"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
)

func (s *Service) checkAndPublish(
	userID int64,
	operationType string,
	currency domain.Currency,
	amountMinor int64,
	opErr error,
) {
	if s.publisher == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	amountRubMinor, err := s.amountRubMinor(ctx, currency, amountMinor)
	if err != nil {
		s.logOperationEventSkipped(err, userID, operationType, currency, amountMinor, opErr)
		return
	}

	status := events.OperationStatusSuccess
	errorMessage := ""
	if opErr != nil {
		status = events.OperationStatusFailed
		errorMessage = opErr.Error()
	}

	createdAt := time.Now().UTC()

	operationEvent := events.OperationEvent{
		EventID:        uuid.NewString(),
		UserID:         userID,
		OperationType:  operationType,
		Status:         status,
		Currency:       string(currency),
		AmountMinor:    amountMinor,
		AmountRubMinor: amountRubMinor,
		CreatedAt:      createdAt,
		Error:          errorMessage,
	}

	if err = s.publisher.PublishOperation(ctx, operationEvent); err != nil {
		s.logOperationPublishError(err, operationEvent)
	}

	if amountRubMinor < s.largeOperationThresholdRubMinor || opErr != nil {
		return
	}

	largeEvent := events.LargeOperationEvent{
		EventID:        uuid.NewString(),
		UserID:         userID,
		OperationType:  operationType,
		Currency:       string(currency),
		AmountMinor:    amountMinor,
		AmountRubMinor: amountRubMinor,
		CreatedAt:      createdAt,
	}

	if err = s.publisher.PublishLargeOperation(ctx, largeEvent); err != nil {
		s.logPublishError(err, largeEvent)
	}
}

func (s *Service) logPublishError(err error, event events.LargeOperationEvent) {
	if s.log == nil {
		return
	}

	s.log.Error("publish large operation failed", map[string]interface{}{
		"error":            err.Error(),
		"event_id":         event.EventID,
		"user_id":          event.UserID,
		"operation_type":   event.OperationType,
		"currency":         event.Currency,
		"amount_minor":     event.AmountMinor,
		"amount_rub_minor": event.AmountRubMinor,
	})
}

func (s *Service) logOperationPublishError(err error, event events.OperationEvent) {
	if s.log == nil {
		return
	}

	s.log.Error("publish operation failed", map[string]interface{}{
		"error":            err.Error(),
		"event_id":         event.EventID,
		"user_id":          event.UserID,
		"operation_type":   event.OperationType,
		"status":           event.Status,
		"currency":         event.Currency,
		"amount_minor":     event.AmountMinor,
		"amount_rub_minor": event.AmountRubMinor,
	})
}

func (s *Service) logOperationEventSkipped(
	err error,
	userID int64,
	operationType string,
	currency domain.Currency,
	amountMinor int64,
	opErr error,
) {
	if s.log == nil {
		return
	}

	fields := map[string]interface{}{
		"error":          err.Error(),
		"user_id":        userID,
		"operation_type": operationType,
		"currency":       string(currency),
		"amount_minor":   amountMinor,
	}
	if opErr != nil {
		fields["operation_error"] = opErr.Error()
	}

	s.log.Error("operation event skipped: failed to calculate RUB amount", fields)
}

func (s *Service) amountRubMinor(ctx context.Context, currency domain.Currency, amountMinor int64) (int64, error) {
	if currency == domain.CurrencyRUB {
		return amountMinor, nil
	}

	if s.exchanger == nil {
		return 0, domain.ErrExchangeRateUnavailable
	}

	rate, ok := s.cachedRate(currency, domain.CurrencyRUB)
	if !ok {
		var err error
		rate, err = s.exchanger.GetRate(ctx, string(currency), string(domain.CurrencyRUB))
		if err != nil {
			return 0, fmt.Errorf("get rub exchange rate: %w", err)
		}
		if !isValidRate(rate) {
			return 0, domain.ErrExchangeRateUnavailable
		}
	}

	return convertMinor(amountMinor, rate), nil
}
