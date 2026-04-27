package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	GRPC     GRPC     `mapstructure:",squash"`
	Postgres Postgres `mapstructure:",squash"`
	Logger   Logger   `mapstructure:",squash"`
}

type GRPC struct {
	Host string `mapstructure:"GRPC_HOST"`
	Port int    `mapstructure:"GRPC_PORT"`
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
	if c.GRPC.Host == "" {
		return fmt.Errorf("GRPC_HOST is required")
	}
	if c.GRPC.Port <= 0 {
		return fmt.Errorf("GRPC_PORT must be greater than 0")
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

func (g *GRPC) Address() string {
	return fmt.Sprintf("%s:%d", g.Host, g.Port)
}
