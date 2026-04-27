package config

import "github.com/spf13/viper"

const DefaultConfigPath = "config.env"

func setDefaults(v *viper.Viper) {
	v.SetDefault("GRPC_HOST", "0.0.0.0")
	v.SetDefault("GRPC_PORT", 50051)

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_USER", "postgres")
	v.SetDefault("POSTGRES_PASSWORD", "postgres")
	v.SetDefault("POSTGRES_DB", "exchange")
	v.SetDefault("POSTGRES_SSLMODE", "disable")
	v.SetDefault("POSTGRES_MAX_OPEN_CONNS", 20)
	v.SetDefault("POSTGRES_MAX_IDLE_CONNS", 10)

	v.SetDefault("LOG_LEVEL", "info")
}
