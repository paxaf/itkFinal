package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/config"
	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	"github.com/paxaf/itkFinal/gw-notification/internal/usecase"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
)

type ConsumerSuite struct {
	suite.Suite

	ctx      context.Context
	handler  *fakeHandler
	consumer *Consumer
}

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.handler = &fakeHandler{}
	s.consumer = &Consumer{handler: s.handler, batchSize: 1, batchWait: time.Millisecond}
}

func (s *ConsumerSuite) TestHandleMessagesDecodesValidMessagesAndSkipsBrokenJSON() {
	event1 := validEvent("event-1")
	event2 := validEvent("event-2")
	messages := []kafkago.Message{
		messageFromEvent(s.T(), event1),
		{Value: []byte("{broken")},
		messageFromEvent(s.T(), event2),
	}

	err := s.consumer.handleMessages(s.ctx, messages)

	s.Require().NoError(err)
	s.Require().True(s.handler.called)
	s.Require().Equal([]domain.LargeOperationEvent{event1, event2}, s.handler.events)
}

func (s *ConsumerSuite) TestHandleMessagesLogsResultAndDecodeFailures() {
	log := &fakeLogger{}
	s.consumer.log = log
	event := validEvent("event-1")

	err := s.consumer.handleMessages(s.ctx, []kafkago.Message{
		messageFromEvent(s.T(), event),
		{Topic: "topic", Partition: 2, Offset: 10, Value: []byte("{broken}")},
	})

	s.Require().NoError(err)
	s.Require().Len(log.warns, 1)
	s.Require().Equal("decode kafka message failed", log.warns[0].message)
	s.Require().Equal("topic", log.warns[0].fields["topic"])
	s.Require().Len(log.infos, 1)
	s.Require().Equal("large operation batch handled", log.infos[0].message)
	s.Require().Equal(2, log.infos[0].fields["messages"])
	s.Require().Equal(1, log.infos[0].fields["decode_failed"])
	s.Require().Equal(1, log.infos[0].fields["accepted"])
}

func (s *ConsumerSuite) TestHandleMessagesSkipsHandlerWhenNothingDecoded() {
	err := s.consumer.handleMessages(s.ctx, []kafkago.Message{
		{Value: []byte("{broken")},
	})

	s.Require().NoError(err)
	s.Require().False(s.handler.called)
}

func (s *ConsumerSuite) TestHandleMessagesReturnsHandlerError() {
	s.handler.err = errHandler

	err := s.consumer.handleMessages(s.ctx, []kafkago.Message{
		messageFromEvent(s.T(), validEvent("event-1")),
	})

	s.Require().ErrorIs(err, errHandler)
}

func (s *ConsumerSuite) TestNewSetsBatchConfig() {
	consumer := New(config.Kafka{
		Brokers:     "localhost:9092",
		Topic:       "wallet.large-operations",
		GroupID:     "gw-notification-test",
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
	s.consumer.reader = &fakeReader{
		fetchResults: []fetchResult{{err: context.Canceled}},
	}

	err := s.consumer.Run(s.ctx)

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TestRunHandlesAndCommitsMessage() {
	event := validEvent("event-1")
	reader := &fakeReader{
		fetchResults: []fetchResult{
			{message: messageFromEvent(s.T(), event)},
			{err: context.Canceled},
		},
	}
	s.consumer.reader = reader

	err := s.consumer.Run(s.ctx)

	s.Require().NoError(err)
	s.Require().Equal([]domain.LargeOperationEvent{event}, s.handler.events)
	s.Require().Len(reader.committedMessages, 1)
}

func (s *ConsumerSuite) TestRunReturnsWhenHandleFailsWithoutCommit() {
	s.handler.err = errHandler
	reader := &fakeReader{
		fetchResults: []fetchResult{
			{message: messageFromEvent(s.T(), validEvent("event-1"))},
		},
	}
	s.consumer.reader = reader

	err := s.consumer.Run(s.ctx)

	s.Require().ErrorIs(err, errHandler)
	s.Require().Empty(reader.committedMessages)
}

func (s *ConsumerSuite) TestRunContinuesAfterCommitError() {
	event := validEvent("event-1")
	log := &fakeLogger{}
	reader := &fakeReader{
		fetchResults: []fetchResult{
			{message: messageFromEvent(s.T(), event)},
			{err: context.Canceled},
		},
		commitErr: errCommit,
	}
	s.consumer.reader = reader
	s.consumer.log = log

	err := s.consumer.Run(s.ctx)

	s.Require().NoError(err)
	s.Require().Equal([]domain.LargeOperationEvent{event}, s.handler.events)
	s.Require().Len(reader.committedMessages, 1)
	s.Require().Len(log.errors, 1)
	s.Require().Equal("commit kafka batch failed", log.errors[0].message)
}

func (s *ConsumerSuite) TestFetchBatchCollectsMessagesUpToBatchSize() {
	reader := &fakeReader{
		fetchResults: []fetchResult{
			{message: kafkago.Message{Value: []byte("1")}},
			{message: kafkago.Message{Value: []byte("2")}},
			{message: kafkago.Message{Value: []byte("3")}},
		},
	}
	s.consumer.reader = reader
	s.consumer.batchSize = 3
	s.consumer.batchWait = time.Second

	messages, err := s.consumer.fetchBatch(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(messages, 3)
}

func (s *ConsumerSuite) TestFetchBatchReturnsFirstMessageWhenBatchWaitEnds() {
	reader := &fakeReader{
		fetchResults: []fetchResult{
			{message: kafkago.Message{Value: []byte("1")}},
			{err: context.DeadlineExceeded},
		},
	}
	s.consumer.reader = reader
	s.consumer.batchSize = 3
	s.consumer.batchWait = time.Second

	messages, err := s.consumer.fetchBatch(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(messages, 1)
}

type fakeHandler struct {
	called bool
	events []domain.LargeOperationEvent
	err    error
}

func (f *fakeHandler) HandleLargeOperation(ctx context.Context, event domain.LargeOperationEvent) (usecase.HandleLargeOperationsResult, error) {
	return f.HandleLargeOperations(ctx, []domain.LargeOperationEvent{event})
}

func (f *fakeHandler) HandleLargeOperations(ctx context.Context, events []domain.LargeOperationEvent) (usecase.HandleLargeOperationsResult, error) {
	f.called = true
	f.events = append([]domain.LargeOperationEvent(nil), events...)
	return usecase.HandleLargeOperationsResult{
		Received: len(events),
		Valid:    len(events),
		Accepted: len(events),
	}, f.err
}

var errHandler = errors.New("handler failed")
var errCommit = errors.New("commit failed")

type fetchResult struct {
	message kafkago.Message
	err     error
}

type fakeReader struct {
	fetchResults      []fetchResult
	commitErr         error
	committedMessages []kafkago.Message
	closed            bool
}

type logEntry struct {
	message string
	fields  map[string]interface{}
}

type fakeLogger struct {
	debugs []logEntry
	infos  []logEntry
	warns  []logEntry
	errors []logEntry
	fatals []logEntry
}

func (l *fakeLogger) Debug(message interface{}, args ...interface{}) {
	l.debugs = append(l.debugs, logEntry{message: stringify(message), fields: fieldsFromArgs(args...)})
}

func (l *fakeLogger) Info(message string, args ...interface{}) {
	l.infos = append(l.infos, logEntry{message: message, fields: fieldsFromArgs(args...)})
}

func (l *fakeLogger) Warn(message string, args ...interface{}) {
	l.warns = append(l.warns, logEntry{message: message, fields: fieldsFromArgs(args...)})
}

func (l *fakeLogger) Error(message interface{}, args ...interface{}) {
	l.errors = append(l.errors, logEntry{message: stringify(message), fields: fieldsFromArgs(args...)})
}

func (l *fakeLogger) Fatal(message interface{}, args ...interface{}) {
	l.fatals = append(l.fatals, logEntry{message: stringify(message), fields: fieldsFromArgs(args...)})
}

func stringify(message interface{}) string {
	value, ok := message.(string)
	if ok {
		return value
	}
	return ""
}

func fieldsFromArgs(args ...interface{}) map[string]interface{} {
	if len(args) == 1 {
		fields, ok := args[0].(map[string]interface{})
		if ok {
			return fields
		}
	}

	return nil
}

func (f *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	if len(f.fetchResults) == 0 {
		return kafkago.Message{}, context.Canceled
	}

	result := f.fetchResults[0]
	f.fetchResults = f.fetchResults[1:]
	return result.message, result.err
}

func (f *fakeReader) CommitMessages(ctx context.Context, msgs ...kafkago.Message) error {
	f.committedMessages = append(f.committedMessages, msgs...)
	return f.commitErr
}

func (f *fakeReader) Close() error {
	f.closed = true
	return nil
}

func messageFromEvent(t *testing.T, event domain.LargeOperationEvent) kafkago.Message {
	t.Helper()

	value, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	return kafkago.Message{Value: value}
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
