package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP      HTTP      `mapstructure:",squash"`
	Postgres  Postgres  `mapstructure:",squash"`
	Auth      Auth      `mapstructure:",squash"`
	Exchanger Exchanger `mapstructure:",squash"`
	Kafka     Kafka     `mapstructure:",squash"`
	Logger    Logger    `mapstructure:",squash"`
}

type HTTP struct {
	Host      string `mapstructure:"HTTP_HOST"`
	Port      int    `mapstructure:"HTTP_PORT"`
	AccessLog bool   `mapstructure:"HTTP_ACCESS_LOG"`
}

type Postgres struct {
	Host         string `mapstructure:"POSTGRES_HOST"`
	Port         int    `mapstructure:"POSTGRES_PORT"`
	User         string `mapstructure:"POSTGRES_USER"`
	Password     string `mapstructure:"POSTGRES_PASSWORD"`
	Name         string `mapstructure:"POSTGRES_DB"`
	SSLMode      string `mapstructure:"POSTGRES_SSLMODE"`
	MaxOpenConns int    `mapstructure:"POSTGRES_MAX_OPEN_CONNS"`
	MaxIdleConns int    `mapstructure:"POSTGRES_MAX_IDLE_CONNS"`
}

type Auth struct {
	JWTSecret       string `mapstructure:"JWT_SECRET"`
	TokenTTLMinutes int    `mapstructure:"JWT_TOKEN_TTL_MINUTES"`
}

type Exchanger struct {
	Host             string `mapstructure:"EXCHANGER_GRPC_HOST"`
	Port             int    `mapstructure:"EXCHANGER_GRPC_PORT"`
	RequestTimeoutMS int    `mapstructure:"EXCHANGER_GRPC_REQUEST_TIMEOUT_MS"`
}

type Kafka struct {
	Brokers                         string `mapstructure:"KAFKA_BROKERS"`
	OperationsTopic                 string `mapstructure:"KAFKA_OPERATIONS_TOPIC"`
	LargeOperationsTopic            string `mapstructure:"KAFKA_LARGE_OPERATIONS_TOPIC"`
	LargeOperationThresholdRubMinor int64  `mapstructure:"LARGE_OPERATION_THRESHOLD_RUB_MINOR"`
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
	if c.HTTP.Host == "" {
		return fmt.Errorf("HTTP_HOST is required")
	}
	if c.HTTP.Port <= 0 {
		return fmt.Errorf("HTTP_PORT must be greater than 0")
	}

	if c.Postgres.Host == "" {
		return fmt.Errorf("POSTGRES_HOST is required")
	}
	if c.Postgres.Port <= 0 {
		return fmt.Errorf("POSTGRES_PORT must be greater than 0")
	}
	if c.Postgres.User == "" || c.Postgres.Password == "" || c.Postgres.Name == "" {
		return fmt.Errorf("POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB are required")
	}
	if c.Postgres.MaxOpenConns <= 0 {
		return fmt.Errorf("POSTGRES_MAX_OPEN_CONNS must be greater than 0")
	}
	if c.Postgres.MaxIdleConns < 0 {
		return fmt.Errorf("POSTGRES_MAX_IDLE_CONNS must be greater than or equal to 0")
	}
	if strings.TrimSpace(c.Auth.JWTSecret) == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.Auth.TokenTTLMinutes <= 0 {
		return fmt.Errorf("JWT_TOKEN_TTL_MINUTES must be greater than 0")
	}

	if strings.TrimSpace(c.Exchanger.Host) == "" {
		return fmt.Errorf("EXCHANGER_GRPC_HOST is required")
	}
	if c.Exchanger.Port <= 0 {
		return fmt.Errorf("EXCHANGER_GRPC_PORT must be greater than 0")
	}
	if c.Exchanger.RequestTimeoutMS <= 0 {
		return fmt.Errorf("EXCHANGER_GRPC_REQUEST_TIMEOUT_MS must be greater than 0")
	}

	if len(c.Kafka.BrokerList()) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if strings.TrimSpace(c.Kafka.OperationsTopic) == "" {
		return fmt.Errorf("KAFKA_OPERATIONS_TOPIC is required")
	}
	if strings.TrimSpace(c.Kafka.LargeOperationsTopic) == "" {
		return fmt.Errorf("KAFKA_LARGE_OPERATIONS_TOPIC is required")
	}
	if c.Kafka.LargeOperationThresholdRubMinor <= 0 {
		return fmt.Errorf("LARGE_OPERATION_THRESHOLD_RUB_MINOR must be greater than 0")
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

func (p *Postgres) DSN() string {
	sslMode := p.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host,
		p.Port,
		p.User,
		p.Password,
		p.Name,
		sslMode,
	)
}

func (h *HTTP) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

func (a *Auth) TokenTTL() time.Duration {
	return time.Duration(a.TokenTTLMinutes) * time.Minute
}

func (e *Exchanger) Address() string {
	return fmt.Sprintf("%s:%d", e.Host, e.Port)
}

func (e *Exchanger) RequestTimeout() time.Duration {
	return time.Duration(e.RequestTimeoutMS) * time.Millisecond
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
