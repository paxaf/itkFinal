package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-analytics/internal/config"
	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
	loggermocks "github.com/paxaf/itkFinal/gw-analytics/internal/mocks/logger"
	kafkamocks "github.com/paxaf/itkFinal/gw-analytics/internal/mocks/transport/kafka"
	usecasemocks "github.com/paxaf/itkFinal/gw-analytics/internal/mocks/usecase"
	"github.com/paxaf/itkFinal/gw-analytics/internal/usecase"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConsumerSuite struct {
	suite.Suite

	ctx      context.Context
	handler  *usecasemocks.OperationHandlerMock
	reader   *kafkamocks.ReaderMock
	consumer *Consumer
}

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.handler = usecasemocks.NewOperationHandlerMock(s.T())
	s.reader = kafkamocks.NewReaderMock(s.T())
	s.consumer = &Consumer{reader: s.reader, handler: s.handler, batchSize: 1, batchWait: time.Millisecond}
}

func (s *ConsumerSuite) TestHandleMessagesDecodesValidMessagesAndSkipsBrokenJSON() {
	event1 := validEvent("event-1")
	event2 := validEvent("event-2")
	messages := []kafkago.Message{
		messageFromEvent(s.T(), event1),
		{Value: []byte("{broken}")},
		messageFromEvent(s.T(), event2),
	}
	s.handler.EXPECT().HandleOperations(s.ctx, []domain.OperationEvent{event1, event2}).Return(usecase.HandleOperationsResult{
		Received: 2,
		Valid:    2,
		Accepted: 2,
	}, nil).Once()

	err := s.consumer.handleMessages(s.ctx, messages)

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestHandleMessagesLogsResultAndDecodeFailures() {
	log := loggermocks.NewLoggerMock(s.T())
	s.consumer.log = log
	event := validEvent("event-1")
	messages := []kafkago.Message{
		messageFromEvent(s.T(), event),
		{Topic: "topic", Partition: 2, Offset: 10, Value: []byte("{broken}")},
	}

	log.EXPECT().Warn("decode kafka message failed", mock.MatchedBy(func(fields map[string]interface{}) bool {
		return fields["topic"] == "topic" &&
			fields["partition"] == 2 &&
			fields["offset"] == int64(10) &&
			fields["error"] != ""
	})).Once()
	s.handler.EXPECT().HandleOperations(s.ctx, []domain.OperationEvent{event}).Return(usecase.HandleOperationsResult{
		Received: 1,
		Valid:    1,
		Accepted: 1,
	}, nil).Once()
	log.EXPECT().Info("operation batch handled", mock.MatchedBy(func(fields map[string]interface{}) bool {
		return fields["messages"] == 2 &&
			fields["decode_failed"] == 1 &&
			fields["accepted"] == 1
	})).Once()

	err := s.consumer.handleMessages(s.ctx, messages)

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestHandleMessagesSkipsHandlerWhenNothingDecoded() {
	err := s.consumer.handleMessages(s.ctx, []kafkago.Message{
		{Value: []byte("{broken}")},
	})

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestHandleMessagesReturnsHandlerError() {
	event := validEvent("event-1")
	s.handler.EXPECT().HandleOperations(s.ctx, []domain.OperationEvent{event}).Return(usecase.HandleOperationsResult{}, errHandler).Once()

	err := s.consumer.handleMessages(s.ctx, []kafkago.Message{messageFromEvent(s.T(), event)})

	s.Require().ErrorIs(err, errHandler)
}

func (s *ConsumerSuite) TestNewSetsBatchConfig() {
	consumer := New(config.Kafka{
		Brokers:     "localhost:9092",
		Topic:       "wallet.operations",
		GroupID:     "gw-analytics-test",
		MinBytes:    1,
		MaxBytes:    1024,
		MaxWaitMS:   10,
		BatchSize:   7,
		BatchWaitMS: 25,
	}, s.handler, nil)
	defer func() { _ = consumer.Close() }()

	s.Require().Equal(7, consumer.batchSize)
	s.Require().Equal(25*time.Millisecond, consumer.batchWait)
	s.Require().NotNil(consumer.reader)
}

func (s *ConsumerSuite) TestCloseHandlesNilReader() {
	consumer := &Consumer{}

	s.Require().NoError(consumer.Close())
}

func (s *ConsumerSuite) TestRunStopsOnCanceledFetch() {
	s.reader.EXPECT().FetchMessage(s.ctx).Return(kafkago.Message{}, context.Canceled).Once()

	err := s.consumer.Run(s.ctx)

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestRunHandlesAndCommitsMessage() {
	event := validEvent("event-1")
	msg := messageFromEvent(s.T(), event)
	s.reader.EXPECT().FetchMessage(s.ctx).Return(msg, nil).Once()
	s.handler.EXPECT().HandleOperations(s.ctx, []domain.OperationEvent{event}).Return(usecase.HandleOperationsResult{
		Received: 1,
		Valid:    1,
		Accepted: 1,
	}, nil).Once()
	s.reader.EXPECT().CommitMessages(s.ctx, msg).Return(nil).Once()
	s.reader.EXPECT().FetchMessage(s.ctx).Return(kafkago.Message{}, context.Canceled).Once()

	err := s.consumer.Run(s.ctx)

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestRunReturnsWhenHandleFailsWithoutCommit() {
	event := validEvent("event-1")
	msg := messageFromEvent(s.T(), event)
	s.reader.EXPECT().FetchMessage(s.ctx).Return(msg, nil).Once()
	s.handler.EXPECT().HandleOperations(s.ctx, []domain.OperationEvent{event}).Return(usecase.HandleOperationsResult{}, errHandler).Once()

	err := s.consumer.Run(s.ctx)

	s.Require().ErrorIs(err, errHandler)
}

func (s *ConsumerSuite) TestRunContinuesAfterCommitError() {
	event := validEvent("event-1")
	msg := messageFromEvent(s.T(), event)
	log := loggermocks.NewLoggerMock(s.T())
	s.consumer.log = log

	s.reader.EXPECT().FetchMessage(s.ctx).Return(msg, nil).Once()
	s.handler.EXPECT().HandleOperations(s.ctx, []domain.OperationEvent{event}).Return(usecase.HandleOperationsResult{
		Received: 1,
		Valid:    1,
		Accepted: 1,
	}, nil).Once()
	log.EXPECT().Info("operation batch handled", mock.MatchedBy(func(fields map[string]interface{}) bool {
		return fields["messages"] == 1 &&
			fields["accepted"] == 1 &&
			fields["decode_failed"] == 0
	})).Once()
	s.reader.EXPECT().CommitMessages(s.ctx, msg).Return(errCommit).Once()
	log.EXPECT().Error("commit kafka batch failed", mock.MatchedBy(func(fields map[string]interface{}) bool {
		return fields["error"] == errCommit.Error()
	})).Once()
	s.reader.EXPECT().FetchMessage(s.ctx).Return(kafkago.Message{}, context.Canceled).Once()

	err := s.consumer.Run(s.ctx)

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestFetchBatchCollectsMessagesUpToBatchSize() {
	s.consumer.batchSize = 3
	s.consumer.batchWait = time.Second
	s.reader.EXPECT().FetchMessage(s.ctx).Return(kafkago.Message{Value: []byte("1")}, nil).Once()
	s.reader.EXPECT().FetchMessage(mock.Anything).Return(kafkago.Message{Value: []byte("2")}, nil).Once()
	s.reader.EXPECT().FetchMessage(mock.Anything).Return(kafkago.Message{Value: []byte("3")}, nil).Once()

	messages, err := s.consumer.fetchBatch(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(messages, 3)
}

func (s *ConsumerSuite) TestFetchBatchReturnsFirstMessageWhenBatchWaitEnds() {
	s.consumer.batchSize = 3
	s.consumer.batchWait = time.Second
	s.reader.EXPECT().FetchMessage(s.ctx).Return(kafkago.Message{Value: []byte("1")}, nil).Once()
	s.reader.EXPECT().FetchMessage(mock.Anything).Return(kafkago.Message{}, context.DeadlineExceeded).Once()

	messages, err := s.consumer.fetchBatch(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(messages, 1)
}

var errHandler = errors.New("handler failed")
var errCommit = errors.New("commit failed")

func messageFromEvent(t *testing.T, event domain.OperationEvent) kafkago.Message {
	t.Helper()

	value, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	return kafkago.Message{Value: value}
}

func validEvent(eventID string) domain.OperationEvent {
	return domain.OperationEvent{
		EventID:        eventID,
		UserID:         42,
		OperationType:  "DEPOSIT",
		Currency:       "RUB",
		AmountMinor:    5_000_000,
		AmountRubMinor: 5_000_000,
		CreatedAt:      time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	}
}
