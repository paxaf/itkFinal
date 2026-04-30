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
	s.Require().Equal("wallet.large-operations", cfg.Kafka.Topic)
	s.Require().Equal(128, cfg.Kafka.BatchSize)
	s.Require().Equal(50*time.Millisecond, cfg.Kafka.BatchWait())
	s.Require().Equal("notification", cfg.Mongo.Database)
	s.Require().Equal("large_operations", cfg.Mongo.Collection)
	s.Require().Equal("info", cfg.Logger.Level)
}

func (s *ConfigSuite) TestLoadFileOverridesDefaults() {
	path := s.writeConfig(`
KAFKA_BROKERS=broker-1:9092, broker-2:9092
KAFKA_TOPIC=events
KAFKA_GROUP_ID=test-group
KAFKA_BATCH_SIZE=64
KAFKA_BATCH_WAIT_MS=25
MONGO_URI=mongodb://example:27017
MONGO_DB=test_db
MONGO_COLLECTION=test_collection
LOG_LEVEL=DEBUG
`)

	cfg, err := Load(path)

	s.Require().NoError(err)
	s.Require().Equal([]string{"broker-1:9092", "broker-2:9092"}, cfg.Kafka.BrokerList())
	s.Require().Equal("events", cfg.Kafka.Topic)
	s.Require().Equal("test-group", cfg.Kafka.GroupID)
	s.Require().Equal(64, cfg.Kafka.BatchSize)
	s.Require().Equal(25*time.Millisecond, cfg.Kafka.BatchWait())
	s.Require().Equal("mongodb://example:27017", cfg.Mongo.ConnectionURI())
	s.Require().Equal("test_db", cfg.Mongo.Database)
	s.Require().Equal("test_collection", cfg.Mongo.Collection)
	s.Require().Equal("debug", cfg.Logger.Level)
}

func (s *ConfigSuite) TestLoadValidationErrors() {
	path := s.writeConfig("LOG_LEVEL=nope\n")

	_, err := Load(path)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "LOG_LEVEL")
}

func (s *ConfigSuite) TestMongoConnectionURIFromParts() {
	cfg := Mongo{
		Host:       "localhost",
		Port:       27017,
		User:       "mongo",
		Password:   "mongo",
		AuthSource: "admin",
	}

	s.Require().Equal("mongodb://mongo:mongo@localhost:27017/?authSource=admin", cfg.ConnectionURI())
}

func (s *ConfigSuite) writeConfig(content string) string {
	path := filepath.Join(s.T().TempDir(), "config.env")
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
	return path
}
