package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
	storagesmocks "github.com/paxaf/itkFinal/gw-analytics/internal/mocks/storages"
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

func (s *UseCaseSuite) TestHandleOperation() {
	event := validEvent("event-1")
	s.storage.EXPECT().SaveOperations(s.ctx, []domain.OperationEvent{event}).Return(nil).Once()

	result, err := s.service.HandleOperation(s.ctx, event)

	s.Require().NoError(err)
	s.Require().Equal(HandleOperationsResult{
		Received: 1,
		Valid:    1,
		Invalid:  0,
		Accepted: 1,
	}, result)
}

func (s *UseCaseSuite) TestHandleOperationsFiltersInvalidEvents() {
	valid := validEvent("event-1")
	invalidUser := validEvent("event-2")
	invalidUser.UserID = 0
	invalidAmount := validEvent("event-3")
	invalidAmount.AmountMinor = 0

	s.storage.EXPECT().SaveOperations(s.ctx, []domain.OperationEvent{valid}).Return(nil).Once()

	result, err := s.service.HandleOperations(s.ctx, []domain.OperationEvent{
		valid,
		invalidUser,
		invalidAmount,
	})

	s.Require().NoError(err)
	s.Require().Equal(HandleOperationsResult{
		Received: 3,
		Valid:    1,
		Invalid:  2,
		Accepted: 1,
	}, result)
}

func (s *UseCaseSuite) TestHandleOperationsSkipsStorageWhenAllEventsInvalid() {
	invalid := validEvent("event-1")
	invalid.EventID = ""

	result, err := s.service.HandleOperations(s.ctx, []domain.OperationEvent{invalid})

	s.Require().NoError(err)
	s.Require().Equal(HandleOperationsResult{
		Received: 1,
		Valid:    0,
		Invalid:  1,
		Accepted: 0,
	}, result)
}

func (s *UseCaseSuite) TestHandleOperationsReturnsStorageError() {
	event := validEvent("event-1")
	s.storage.EXPECT().SaveOperations(s.ctx, mock.MatchedBy(func(events []domain.OperationEvent) bool {
		return len(events) == 1 && events[0] == event
	})).Return(errBoom).Once()

	result, err := s.service.HandleOperations(s.ctx, []domain.OperationEvent{event})

	s.Require().ErrorIs(err, errBoom)
	s.Require().Equal(HandleOperationsResult{
		Received: 1,
		Valid:    1,
		Invalid:  0,
		Accepted: 0,
	}, result)
}

func validEvent(eventID string) domain.OperationEvent {
	return domain.OperationEvent{
		EventID:        eventID,
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
