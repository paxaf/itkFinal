package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) TestLoadUsesDefaultsWhenFileMissing() {
	cfg, err := Load(filepath.Join(s.T().TempDir(), "missing.env"))

	s.Require().NoError(err)
	s.Require().Equal("0.0.0.0", cfg.GRPC.Host)
	s.Require().Equal(50051, cfg.GRPC.Port)
	s.Require().Equal("localhost", cfg.Postgres.Host)
	s.Require().Equal(5432, cfg.Postgres.Port)
	s.Require().Equal("postgres", cfg.Postgres.User)
	s.Require().Equal("exchange", cfg.Postgres.Name)
	s.Require().Equal("info", cfg.Logger.Level)
}

func (s *ConfigSuite) TestLoadReadsEnvFileAndNormalizesLogLevel() {
	path := filepath.Join(s.T().TempDir(), "config.env")
	err := os.WriteFile(path, []byte(`
GRPC_HOST=127.0.0.1
GRPC_PORT=60051
POSTGRES_HOST=db
POSTGRES_PORT=5433
POSTGRES_USER=user
POSTGRES_PASSWORD=secret
POSTGRES_DB=rates
POSTGRES_SSLMODE=require
POSTGRES_MAX_OPEN_CONNS=7
POSTGRES_MAX_IDLE_CONNS=2
LOG_LEVEL=DEBUG
`), 0o600)
	s.Require().NoError(err)

	cfg, err := Load(path)

	s.Require().NoError(err)
	s.Require().Equal("127.0.0.1", cfg.GRPC.Host)
	s.Require().Equal(60051, cfg.GRPC.Port)
	s.Require().Equal("db", cfg.Postgres.Host)
	s.Require().Equal(5433, cfg.Postgres.Port)
	s.Require().Equal("user", cfg.Postgres.User)
	s.Require().Equal("secret", cfg.Postgres.Password)
	s.Require().Equal("rates", cfg.Postgres.Name)
	s.Require().Equal("require", cfg.Postgres.SSLMode)
	s.Require().Equal(7, cfg.Postgres.MaxOpenConns)
	s.Require().Equal(2, cfg.Postgres.MaxIdleConns)
	s.Require().Equal("debug", cfg.Logger.Level)
}

func (s *ConfigSuite) TestLoadUsesDefaultPathWhenPathEmpty() {
	s.T().Setenv("GRPC_HOST", "0.0.0.0")
	s.T().Setenv("GRPC_PORT", "50051")

	cfg, err := Load("   ")

	s.Require().NoError(err)
	s.Require().Equal("0.0.0.0", cfg.GRPC.Host)
	s.Require().Equal(50051, cfg.GRPC.Port)
}

func (s *ConfigSuite) TestMustLoadPanicsOnInvalidConfig() {
	path := filepath.Join(s.T().TempDir(), "bad.env")
	err := os.WriteFile(path, []byte("GRPC_PORT=0\n"), 0o600)
	s.Require().NoError(err)

	s.Require().Panics(func() {
		_ = MustLoad(path)
	})
}

func (s *ConfigSuite) TestValidateErrors() {
	base := validConfig()

	tests := map[string]struct {
		mutate func(*Config)
		want   string
	}{
		"grpc host": {
			mutate: func(cfg *Config) { cfg.GRPC.Host = "" },
			want:   "GRPC_HOST is required",
		},
		"grpc port": {
			mutate: func(cfg *Config) { cfg.GRPC.Port = 0 },
			want:   "GRPC_PORT must be greater than 0",
		},
		"postgres host": {
			mutate: func(cfg *Config) { cfg.Postgres.Host = "" },
			want:   "POSTGRES_HOST is required",
		},
		"postgres port": {
			mutate: func(cfg *Config) { cfg.Postgres.Port = 0 },
			want:   "POSTGRES_PORT must be greater than 0",
		},
		"postgres credentials": {
			mutate: func(cfg *Config) { cfg.Postgres.User = "" },
			want:   "POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB are required",
		},
		"max open conns": {
			mutate: func(cfg *Config) { cfg.Postgres.MaxOpenConns = 0 },
			want:   "POSTGRES_MAX_OPEN_CONNS must be greater than 0",
		},
		"max idle conns": {
			mutate: func(cfg *Config) { cfg.Postgres.MaxIdleConns = -1 },
			want:   "POSTGRES_MAX_IDLE_CONNS must be greater than or equal to 0",
		},
		"log level": {
			mutate: func(cfg *Config) { cfg.Logger.Level = "trace" },
			want:   "LOG_LEVEL must be one of",
		},
	}

	for name, tt := range tests {
		s.Run(name, func() {
			cfg := base
			tt.mutate(&cfg)

			err := cfg.validate()

			s.Require().Error(err)
			s.Require().Contains(err.Error(), tt.want)
		})
	}
}

func (s *ConfigSuite) TestValidateSetsDefaultLogLevel() {
	cfg := validConfig()
	cfg.Logger.Level = " "

	s.Require().NoError(cfg.validate())
	s.Require().Equal("info", cfg.Logger.Level)
}

func (s *ConfigSuite) TestPostgresDSN() {
	cfg := validConfig().Postgres
	cfg.SSLMode = ""

	s.Require().Equal(
		"host=localhost port=5432 user=postgres password=postgres dbname=exchange sslmode=disable",
		cfg.DSN(),
	)

	cfg.SSLMode = "require"
	s.Require().Contains(cfg.DSN(), "sslmode=require")
}

func (s *ConfigSuite) TestGRPCAddress() {
	cfg := GRPC{Host: "127.0.0.1", Port: 50051}

	s.Require().Equal("127.0.0.1:50051", cfg.Address())
}

func validConfig() Config {
	return Config{
		GRPC: GRPC{
			Host: "0.0.0.0",
			Port: 50051,
		},
		Postgres: Postgres{
			Host:         "localhost",
			Port:         5432,
			User:         "postgres",
			Password:     "postgres",
			Name:         "exchange",
			SSLMode:      "disable",
			MaxOpenConns: 20,
			MaxIdleConns: 10,
		},
		Logger: Logger{
			Level: "info",
		},
	}
}
