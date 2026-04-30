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

func (s *EventSuite) TestLargeOperationEventValidate() {
	event := validEvent()

	s.Require().NoError(event.Validate())
}

func (s *EventSuite) TestLargeOperationEventValidateErrors() {
	tests := []struct {
		name    string
		mutate  func(*LargeOperationEvent)
		wantErr error
	}{
		{
			name: "empty event id",
			mutate: func(event *LargeOperationEvent) {
				event.EventID = " "
			},
			wantErr: ErrInvalidEventID,
		},
		{
			name: "invalid user id",
			mutate: func(event *LargeOperationEvent) {
				event.UserID = 0
			},
			wantErr: ErrInvalidUserID,
		},
		{
			name: "empty operation type",
			mutate: func(event *LargeOperationEvent) {
				event.OperationType = ""
			},
			wantErr: ErrInvalidOperationType,
		},
		{
			name: "empty currency",
			mutate: func(event *LargeOperationEvent) {
				event.Currency = " "
			},
			wantErr: ErrInvalidCurrency,
		},
		{
			name: "invalid amount",
			mutate: func(event *LargeOperationEvent) {
				event.AmountMinor = 0
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "invalid rub amount",
			mutate: func(event *LargeOperationEvent) {
				event.AmountRubMinor = -1
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "zero created at",
			mutate: func(event *LargeOperationEvent) {
				event.CreatedAt = time.Time{}
			},
			wantErr: ErrInvalidCreatedAt,
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

func validEvent() LargeOperationEvent {
	return LargeOperationEvent{
		EventID:        "event-1",
		UserID:         42,
		OperationType:  "DEPOSIT",
		Currency:       "RUB",
		AmountMinor:    5_000_000,
		AmountRubMinor: 5_000_000,
		CreatedAt:      time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	}
}
