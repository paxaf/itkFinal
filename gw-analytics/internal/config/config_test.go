package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) TestLoadMissingFileUsesDefaults() {
	cfg, err := Load(filepath.Join(s.T().TempDir(), "missing.env"))

	s.Require().NoError(err)
	s.Require().Equal([]string{"localhost:9092"}, cfg.Kafka.BrokerList())
	s.Require().Equal("wallet.operations", cfg.Kafka.Topic)
	s.Require().Equal("gw-analytics", cfg.Kafka.GroupID)
	s.Require().Equal(128, cfg.Kafka.BatchSize)
	s.Require().Equal(50*time.Millisecond, cfg.Kafka.BatchWait())
	s.Require().Equal([]string{"http://localhost:9200"}, cfg.Elasticsearch.AddressList())
	s.Require().Equal("wallet_operations", cfg.Elasticsearch.Index)
	s.Require().Equal("info", cfg.Logger.Level)
}

func (s *ConfigSuite) TestLoadFileOverridesDefaults() {
	path := s.writeConfig(`
KAFKA_BROKERS=broker-1:9092, broker-2:9092
KAFKA_TOPIC=events
KAFKA_GROUP_ID=test-group
KAFKA_BATCH_SIZE=64
KAFKA_BATCH_WAIT_MS=25
ELASTIC_ADDRESSES=http://elastic-1:9200, http://elastic-2:9200
ELASTIC_USERNAME=elastic
ELASTIC_PASSWORD=secret
ELASTIC_INDEX=test_operations
LOG_LEVEL=DEBUG
`)

	cfg, err := Load(path)

	s.Require().NoError(err)
	s.Require().Equal([]string{"broker-1:9092", "broker-2:9092"}, cfg.Kafka.BrokerList())
	s.Require().Equal("events", cfg.Kafka.Topic)
	s.Require().Equal("test-group", cfg.Kafka.GroupID)
	s.Require().Equal(64, cfg.Kafka.BatchSize)
	s.Require().Equal(25*time.Millisecond, cfg.Kafka.BatchWait())
	s.Require().Equal([]string{"http://elastic-1:9200", "http://elastic-2:9200"}, cfg.Elasticsearch.AddressList())
	s.Require().Equal("elastic", cfg.Elasticsearch.Username)
	s.Require().Equal("secret", cfg.Elasticsearch.Password)
	s.Require().Equal("test_operations", cfg.Elasticsearch.Index)
	s.Require().Equal("debug", cfg.Logger.Level)
}

func (s *ConfigSuite) TestLoadValidationErrors() {
	path := s.writeConfig("LOG_LEVEL=nope\n")

	_, err := Load(path)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "LOG_LEVEL")
}

func (s *ConfigSuite) writeConfig(content string) string {
	path := filepath.Join(s.T().TempDir(), "config.env")
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
	return path
}
