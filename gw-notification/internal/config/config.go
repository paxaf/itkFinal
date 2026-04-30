package config

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Kafka  Kafka  `mapstructure:",squash"`
	Mongo  Mongo  `mapstructure:",squash"`
	Logger Logger `mapstructure:",squash"`
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

type Mongo struct {
	URI              string `mapstructure:"MONGO_URI"`
	Host             string `mapstructure:"MONGO_HOST"`
	Port             int    `mapstructure:"MONGO_PORT"`
	User             string `mapstructure:"MONGO_USER"`
	Password         string `mapstructure:"MONGO_PASSWORD"`
	AuthSource       string `mapstructure:"MONGO_AUTH_SOURCE"`
	Database         string `mapstructure:"MONGO_DB"`
	Collection       string `mapstructure:"MONGO_COLLECTION"`
	ConnectTimeoutMS int    `mapstructure:"MONGO_CONNECT_TIMEOUT_MS"`
}

type Logger struct {
	Level string `mapstructure:"LOG_LEVEL"`
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

	if strings.TrimSpace(c.Mongo.URI) == "" {
		if strings.TrimSpace(c.Mongo.Host) == "" {
			return fmt.Errorf("MONGO_HOST is required")
		}
		if c.Mongo.Port <= 0 {
			return fmt.Errorf("MONGO_PORT must be greater than 0")
		}
	}
	if strings.TrimSpace(c.Mongo.Database) == "" {
		return fmt.Errorf("MONGO_DB is required")
	}
	if strings.TrimSpace(c.Mongo.Collection) == "" {
		return fmt.Errorf("MONGO_COLLECTION is required")
	}
	if c.Mongo.ConnectTimeoutMS <= 0 {
		return fmt.Errorf("MONGO_CONNECT_TIMEOUT_MS must be greater than 0")
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

func (m *Mongo) ConnectionURI() string {
	if strings.TrimSpace(m.URI) != "" {
		return strings.TrimSpace(m.URI)
	}

	auth := ""
	if strings.TrimSpace(m.User) != "" {
		auth = url.QueryEscape(m.User)
		if strings.TrimSpace(m.Password) != "" {
			auth += ":" + url.QueryEscape(m.Password)
		}
		auth += "@"
	}

	query := ""
	authSource := strings.TrimSpace(m.AuthSource)
	if authSource != "" {
		query = "?authSource=" + url.QueryEscape(authSource)
	}

	return "mongodb://" + auth + strings.TrimSpace(m.Host) + ":" + strconv.Itoa(m.Port) + "/" + query
}

func (m *Mongo) ConnectTimeout() time.Duration {
	return time.Duration(m.ConnectTimeoutMS) * time.Millisecond
}
