package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	storagesmocks "github.com/paxaf/itkFinal/gw-notification/internal/mocks/storages"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var errBoom = errors.New("boom")

type UseCaseSuite struct {
	suite.Suite

	ctx     context.Context
	storage *storagesmocks.StorageMock
	service *Service
}

func TestUseCaseSuite(t *testing.T) {
	suite.Run(t, new(UseCaseSuite))
}

func (s *UseCaseSuite) SetupTest() {
	s.ctx = context.Background()
	s.storage = storagesmocks.NewStorageMock(s.T())
	s.service = New(s.storage)
}

func (s *UseCaseSuite) TestHandleLargeOperation() {
	event := validEvent("event-1")
	s.storage.EXPECT().SaveLargeOperations(s.ctx, []domain.LargeOperationEvent{event}).Return(nil).Once()

	result, err := s.service.HandleLargeOperation(s.ctx, event)

	s.Require().NoError(err)
	s.Require().Equal(HandleLargeOperationsResult{
		Received: 1,
		Valid:    1,
		Invalid:  0,
		Accepted: 1,
	}, result)
}

func (s *UseCaseSuite) TestHandleLargeOperationsFiltersInvalidEvents() {
	valid := validEvent("event-1")
	invalidUser := validEvent("event-2")
	invalidUser.UserID = 0
	invalidAmount := validEvent("event-3")
	invalidAmount.AmountMinor = 0

	s.storage.EXPECT().SaveLargeOperations(s.ctx, []domain.LargeOperationEvent{valid}).Return(nil).Once()

	result, err := s.service.HandleLargeOperations(s.ctx, []domain.LargeOperationEvent{
		valid,
		invalidUser,
		invalidAmount,
	})

	s.Require().NoError(err)
	s.Require().Equal(HandleLargeOperationsResult{
		Received: 3,
		Valid:    1,
		Invalid:  2,
		Accepted: 1,
	}, result)
}

func (s *UseCaseSuite) TestHandleLargeOperationsSkipsStorageWhenAllEventsInvalid() {
	invalid := validEvent("event-1")
	invalid.EventID = ""

	result, err := s.service.HandleLargeOperations(s.ctx, []domain.LargeOperationEvent{invalid})

	s.Require().NoError(err)
	s.Require().Equal(HandleLargeOperationsResult{
		Received: 1,
		Valid:    0,
		Invalid:  1,
		Accepted: 0,
	}, result)
}

func (s *UseCaseSuite) TestHandleLargeOperationsReturnsStorageError() {
	event := validEvent("event-1")
	s.storage.EXPECT().SaveLargeOperations(s.ctx, mock.MatchedBy(func(events []domain.LargeOperationEvent) bool {
		return len(events) == 1 && events[0] == event
	})).Return(errBoom).Once()

	result, err := s.service.HandleLargeOperations(s.ctx, []domain.LargeOperationEvent{event})

	s.Require().ErrorIs(err, errBoom)
	s.Require().Equal(HandleLargeOperationsResult{
		Received: 1,
		Valid:    1,
		Invalid:  0,
		Accepted: 0,
	}, result)
}

func validEvent(eventID string) domain.LargeOperationEvent {
	return domain.LargeOperationEvent{
		EventID:        eventID,
		UserID:         42,
		OperationType:  "DEPOSIT",
		Currency:       "RUB",
		AmountMinor:    5_000_000,
		AmountRubMinor: 5_000_000,
		CreatedAt:      time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	}
}
