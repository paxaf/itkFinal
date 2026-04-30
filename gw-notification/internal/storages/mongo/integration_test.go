package mongo

import (
	"context"
	"flag"
	"os/exec"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/config"
	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var skipIntegration = flag.Bool("skip-integration", false, "skip integration tests")

type MongoIntegrationSuite struct {
	suite.Suite

	ctx       context.Context
	container *tcmongodb.MongoDBContainer
	storage   *DB
}

func TestMongoIntegrationSuite(t *testing.T) {
	if *skipIntegration {
		t.Skip("integration tests are skipped by -skip-integration")
	}
	if testing.Short() {
		t.Skip("integration tests are skipped in short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("integration tests are skipped: docker executable not found")
	}
	tc.SkipIfProviderIsNotHealthy(t)

	suite.Run(t, new(MongoIntegrationSuite))
}

func (s *MongoIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := tcmongodb.Run(s.ctx, "mongo:7.0")
	s.Require().NoError(err)
	s.container = container

	uri, err := container.ConnectionString(s.ctx)
	s.Require().NoError(err)

	storage, err := New(config.Mongo{
		URI:              uri,
		Database:         "notification_test",
		Collection:       "large_operations",
		ConnectTimeoutMS: 5000,
	})
	s.Require().NoError(err)
	s.storage = storage
}

func (s *MongoIntegrationSuite) TearDownSuite() {
	if s.storage != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.storage.Close(closeCtx)
		cancel()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *MongoIntegrationSuite) SetupTest() {
	_, err := s.storage.collection.DeleteMany(s.ctx, bson.D{})
	s.Require().NoError(err)
}

func (s *MongoIntegrationSuite) TestSaveLargeOperations() {
	events := []domain.LargeOperationEvent{
		validEvent("event-1"),
		validEvent("event-2"),
	}

	err := s.storage.SaveLargeOperations(s.ctx, events)

	s.Require().NoError(err)
	count, err := s.storage.collection.CountDocuments(s.ctx, bson.D{})
	s.Require().NoError(err)
	s.Require().Equal(int64(2), count)

	var doc largeOperationDocument
	err = s.storage.collection.FindOne(s.ctx, bson.D{{Key: "event_id", Value: "event-1"}}).Decode(&doc)
	s.Require().NoError(err)
	s.Require().Equal(events[0].UserID, doc.UserID)
	s.Require().Equal(events[0].AmountRubMinor, doc.AmountRubMinor)
}

func (s *MongoIntegrationSuite) TestDuplicateEventsAreIdempotent() {
	event1 := validEvent("event-1")
	event2 := validEvent("event-2")

	s.Require().NoError(s.storage.SaveLargeOperation(s.ctx, event1))
	s.Require().NoError(s.storage.SaveLargeOperations(s.ctx, []domain.LargeOperationEvent{
		event1,
		event2,
	}))

	count, err := s.storage.collection.CountDocuments(s.ctx, bson.D{})
	s.Require().NoError(err)
	s.Require().Equal(int64(2), count)
}

func (s *MongoIntegrationSuite) TestSaveEmptyBatch() {
	s.Require().NoError(s.storage.SaveLargeOperations(s.ctx, nil))

	count, err := s.storage.collection.CountDocuments(s.ctx, bson.D{})
	s.Require().NoError(err)
	s.Require().Zero(count)
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
