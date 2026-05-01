package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type EventSuite struct {
	suite.Suite
}

func TestEventSuite(t *testing.T) {
	suite.Run(t, new(EventSuite))
}

func (s *EventSuite) TestOperationEventValidate() {
	event := validEvent()

	s.Require().NoError(event.Validate())
}

func (s *EventSuite) TestOperationEventValidateFailedRequiresErrorMessage() {
	event := validEvent()
	event.Status = "FAILED"
	event.Error = "insufficient funds"

	s.Require().NoError(event.Validate())
}

func (s *EventSuite) TestOperationEventValidateErrors() {
	tests := []struct {
		name    string
		mutate  func(*OperationEvent)
		wantErr error
	}{
		{
			name: "empty event id",
			mutate: func(event *OperationEvent) {
				event.EventID = " "
			},
			wantErr: ErrInvalidEventID,
		},
		{
			name: "invalid user id",
			mutate: func(event *OperationEvent) {
				event.UserID = 0
			},
			wantErr: ErrInvalidUserID,
		},
		{
			name: "unsupported operation type",
			mutate: func(event *OperationEvent) {
				event.OperationType = "TRANSFER"
			},
			wantErr: ErrInvalidOperationType,
		},
		{
			name: "unsupported status",
			mutate: func(event *OperationEvent) {
				event.Status = "PENDING"
			},
			wantErr: ErrInvalidStatus,
		},
		{
			name: "unsupported currency",
			mutate: func(event *OperationEvent) {
				event.Currency = "GBP"
			},
			wantErr: ErrInvalidCurrency,
		},
		{
			name: "invalid amount",
			mutate: func(event *OperationEvent) {
				event.AmountMinor = 0
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "invalid rub amount",
			mutate: func(event *OperationEvent) {
				event.AmountRubMinor = -1
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "zero created at",
			mutate: func(event *OperationEvent) {
				event.CreatedAt = time.Time{}
			},
			wantErr: ErrInvalidCreatedAt,
		},
		{
			name: "negative retry count",
			mutate: func(event *OperationEvent) {
				event.RetryCount = -1
			},
			wantErr: ErrInvalidRetryCount,
		},
		{
			name: "failed without error",
			mutate: func(event *OperationEvent) {
				event.Status = "FAILED"
			},
			wantErr: ErrInvalidErrorMessage,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			event := validEvent()
			tt.mutate(&event)

			s.Require().ErrorIs(event.Validate(), tt.wantErr)
		})
	}
}

func validEvent() OperationEvent {
	return OperationEvent{
		EventID:        "event-1",
		UserID:         42,
		OperationType:  "DEPOSIT",
		Status:         "SUCCESS",
		Currency:       "RUB",
		AmountMinor:    5_000_000,
		AmountRubMinor: 5_000_000,
		CreatedAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		RetryCount:     0,
	}
}
