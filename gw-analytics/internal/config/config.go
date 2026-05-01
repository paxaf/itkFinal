package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Kafka         Kafka         `mapstructure:",squash"`
	Elasticsearch Elasticsearch `mapstructure:",squash"`
	Logger        Logger        `mapstructure:",squash"`
}

type Kafka struct {
	Brokers     string `mapstructure:"KAFKA_BROKERS"`
	Topic       string `mapstructure:"KAFKA_TOPIC"`
	GroupID     string `mapstructure:"KAFKA_GROUP_ID"`
	MinBytes    int    `mapstructure:"KAFKA_MIN_BYTES"`
	MaxBytes    int    `mapstructure:"KAFKA_MAX_BYTES"`
	MaxWaitMS   int    `mapstructure:"KAFKA_MAX_WAIT_MS"`
	BatchSize   int    `mapstructure:"KAFKA_BATCH_SIZE"`
	BatchWaitMS int    `mapstructure:"KAFKA_BATCH_WAIT_MS"`
}

type Logger struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

type Elasticsearch struct {
	Addresses string `mapstructure:"ELASTIC_ADDRESSES"`
	Username  string `mapstructure:"ELASTIC_USERNAME"`
	Password  string `mapstructure:"ELASTIC_PASSWORD"`
	Index     string `mapstructure:"ELASTIC_INDEX"`
}

func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultConfigPath
	}

	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(path)
	v.SetConfigType("env")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		log.Printf("config: failed to read %q, using defaults/env: %v", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &cfg, nil
}

func MustLoad(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}

	return cfg
}

func (c *Config) validate() error {
	if len(c.Kafka.BrokerList()) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if strings.TrimSpace(c.Kafka.Topic) == "" {
		return fmt.Errorf("KAFKA_TOPIC is required")
	}
	if strings.TrimSpace(c.Kafka.GroupID) == "" {
		return fmt.Errorf("KAFKA_GROUP_ID is required")
	}
	if c.Kafka.MinBytes <= 0 {
		return fmt.Errorf("KAFKA_MIN_BYTES must be greater than 0")
	}
	if c.Kafka.MaxBytes <= 0 {
		return fmt.Errorf("KAFKA_MAX_BYTES must be greater than 0")
	}
	if c.Kafka.MaxWaitMS <= 0 {
		return fmt.Errorf("KAFKA_MAX_WAIT_MS must be greater than 0")
	}
	if c.Kafka.BatchSize <= 0 {
		return fmt.Errorf("KAFKA_BATCH_SIZE must be greater than 0")
	}
	if c.Kafka.BatchWaitMS <= 0 {
		return fmt.Errorf("KAFKA_BATCH_WAIT_MS must be greater than 0")
	}
	if len(c.Elasticsearch.AddressList()) == 0 {
		return fmt.Errorf("ELASTIC_ADDRESSES is required")
	}
	if strings.TrimSpace(c.Elasticsearch.Index) == "" {
		return fmt.Errorf("ELASTIC_INDEX is required")
	}

	level := strings.ToLower(strings.TrimSpace(c.Logger.Level))
	if level == "" {
		level = "info"
	}

	switch level {
	case "debug", "info", "warn", "error", "fatal":
	default:
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error, fatal")
	}

	c.Logger.Level = level
	return nil
}

func (k *Kafka) BrokerList() []string {
	parts := strings.Split(k.Brokers, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		broker := strings.TrimSpace(part)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func (k *Kafka) MaxWait() time.Duration {
	return time.Duration(k.MaxWaitMS) * time.Millisecond
}

func (k *Kafka) BatchWait() time.Duration {
	return time.Duration(k.BatchWaitMS) * time.Millisecond
}

func (e *Elasticsearch) AddressList() []string {
	parts := strings.Split(e.Addresses, ",")
	addresses := make([]string, 0, len(parts))
	for _, part := range parts {
		address := strings.TrimSpace(part)
		if address != "" {
			addresses = append(addresses, address)
		}
	}
	return addresses
}
