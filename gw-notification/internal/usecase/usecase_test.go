package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	"github.com/stretchr/testify/suite"
)

var errBoom = errors.New("boom")

type UseCaseSuite struct {
	suite.Suite

	ctx     context.Context
	storage *fakeStorage
	service *Service
}

func TestUseCaseSuite(t *testing.T) {
	suite.Run(t, new(UseCaseSuite))
}

func (s *UseCaseSuite) SetupTest() {
	s.ctx = context.Background()
	s.storage = &fakeStorage{}
	s.service = New(s.storage)
}

func (s *UseCaseSuite) TestHandleLargeOperation() {
	event := validEvent("event-1")

	result, err := s.service.HandleLargeOperation(s.ctx, event)

	s.Require().NoError(err)
	s.Require().Equal(HandleLargeOperationsResult{
		Received: 1,
		Valid:    1,
		Invalid:  0,
		Accepted: 1,
	}, result)
	s.Require().Equal([]domain.LargeOperationEvent{event}, s.storage.savedEvents)
}

func (s *UseCaseSuite) TestHandleLargeOperationsFiltersInvalidEvents() {
	valid := validEvent("event-1")
	invalidUser := validEvent("event-2")
	invalidUser.UserID = 0
	invalidAmount := validEvent("event-3")
	invalidAmount.AmountMinor = 0

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
	s.Require().Equal([]domain.LargeOperationEvent{valid}, s.storage.savedEvents)
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
	s.Require().False(s.storage.saveManyCalled)
}

func (s *UseCaseSuite) TestHandleLargeOperationsReturnsStorageError() {
	s.storage.saveManyErr = errBoom
	event := validEvent("event-1")

	result, err := s.service.HandleLargeOperations(s.ctx, []domain.LargeOperationEvent{event})

	s.Require().ErrorIs(err, errBoom)
	s.Require().Equal(HandleLargeOperationsResult{
		Received: 1,
		Valid:    1,
		Invalid:  0,
		Accepted: 0,
	}, result)
}

type fakeStorage struct {
	savedEvents    []domain.LargeOperationEvent
	saveManyErr    error
	saveManyCalled bool
}

func (f *fakeStorage) SaveLargeOperations(ctx context.Context, events []domain.LargeOperationEvent) error {
	f.saveManyCalled = true
	f.savedEvents = append([]domain.LargeOperationEvent(nil), events...)
	return f.saveManyErr
}

func (f *fakeStorage) SaveLargeOperation(ctx context.Context, event domain.LargeOperationEvent) error {
	return f.SaveLargeOperations(ctx, []domain.LargeOperationEvent{event})
}

func (f *fakeStorage) Close(ctx context.Context) error {
	return nil
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
