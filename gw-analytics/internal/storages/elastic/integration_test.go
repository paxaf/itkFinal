package elastic

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/paxaf/itkFinal/gw-analytics/internal/config"
	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var skipIntegration = flag.Bool("skip-integration", false, "skip integration tests")

type ElasticIntegrationSuite struct {
	suite.Suite

	ctx       context.Context
	container tc.Container
	address   string
	storage   *Storage
	client    *elasticsearch.TypedClient
}

func TestElasticIntegrationSuite(t *testing.T) {
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

	suite.Run(t, new(ElasticIntegrationSuite))
}

func (s *ElasticIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := tc.GenericContainer(s.ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        elasticImage(),
			ExposedPorts: []string{"9200/tcp"},
			Env: map[string]string{
				"discovery.type":         "single-node",
				"xpack.security.enabled": "false",
				"ES_JAVA_OPTS":           "-Xms512m -Xmx512m",
			},
			WaitingFor: wait.ForHTTP("/").
				WithPort(nat.Port("9200/tcp")).
				WithStartupTimeout(3 * time.Minute),
		},
		Started: true,
	})
	s.Require().NoError(err)
	s.container = container

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)

	port, err := container.MappedPort(s.ctx, nat.Port("9200/tcp"))
	s.Require().NoError(err)

	s.address = fmt.Sprintf("http://%s:%s", host, port.Port())
}

func (s *ElasticIntegrationSuite) TearDownSuite() {
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *ElasticIntegrationSuite) SetupTest() {
	index := fmt.Sprintf("wallet_operations_%d", time.Now().UnixNano())
	storage, err := New(s.ctx, config.Elasticsearch{
		Addresses: s.address,
		Index:     index,
	})
	s.Require().NoError(err)
	s.storage = storage

	typed, ok := storage.client.(*typedClient)
	s.Require().True(ok)
	s.client = typed.client
}

func (s *ElasticIntegrationSuite) TearDownTest() {
	if s.client != nil && s.storage != nil {
		_, _ = s.client.Indices.Delete(s.storage.index).Do(context.Background())
	}
}

func (s *ElasticIntegrationSuite) TestSaveOperationsStoresDocuments() {
	event := integrationEvent("event-1")

	err := s.storage.SaveOperations(s.ctx, []domain.OperationEvent{
		event,
		integrationEvent("event-2"),
	})

	s.Require().NoError(err)

	doc := s.getDocument(event.EventID)
	s.Require().Equal(event.EventID, doc.EventID)
	s.Require().Equal(event.UserID, doc.UserID)
	s.Require().Equal(event.AmountRubMinor, doc.AmountRubMinor)
	s.Require().Equal(1, doc.DeliveryCount)
	s.Require().Zero(doc.DuplicateCount)
}

func (s *ElasticIntegrationSuite) TestDuplicateEventsIncrementCounters() {
	event := integrationEvent("event-1")

	s.Require().NoError(s.storage.SaveOperation(s.ctx, event))
	s.Require().NoError(s.storage.SaveOperations(s.ctx, []domain.OperationEvent{event}))

	doc := s.getDocument(event.EventID)
	s.Require().Equal(2, doc.DeliveryCount)
	s.Require().Equal(1, doc.DuplicateCount)
}

func (s *ElasticIntegrationSuite) TestSaveEmptyBatch() {
	s.Require().NoError(s.storage.SaveOperations(s.ctx, nil))
}

func (s *ElasticIntegrationSuite) getDocument(eventID string) operationDocument {
	resp, err := s.client.Get(s.storage.index, eventID).Do(s.ctx)
	s.Require().NoError(err)
	s.Require().True(resp.Found)

	var doc operationDocument
	s.Require().NoError(json.Unmarshal(resp.Source_, &doc))
	return doc
}

func integrationEvent(eventID string) domain.OperationEvent {
	return domain.OperationEvent{
		EventID:        eventID,
		UserID:         42,
		OperationType:  "DEPOSIT",
		Status:         "SUCCESS",
		Currency:       "RUB",
		AmountMinor:    5_000_000,
		AmountRubMinor: 5_000_000,
		CreatedAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}
}

func elasticImage() string {
	if image := os.Getenv("ELASTIC_IMAGE"); image != "" {
		return image
	}
	return "docker.elastic.co/elasticsearch/elasticsearch:9.3.3"
}
