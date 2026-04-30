package config

import "github.com/spf13/viper"

const DefaultConfigPath = "config.env"

func setDefaults(v *viper.Viper) {
	v.SetDefault("HTTP_HOST", "0.0.0.0")
	v.SetDefault("HTTP_PORT", 8080)
	v.SetDefault("HTTP_ACCESS_LOG", true)

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_USER", "postgres")
	v.SetDefault("POSTGRES_PASSWORD", "postgres")
	v.SetDefault("POSTGRES_DB", "wallet")
	v.SetDefault("POSTGRES_SSLMODE", "disable")
	v.SetDefault("POSTGRES_MAX_OPEN_CONNS", 20)
	v.SetDefault("POSTGRES_MAX_IDLE_CONNS", 10)

	v.SetDefault("JWT_SECRET", "local-dev-secret")
	v.SetDefault("JWT_TOKEN_TTL_MINUTES", 60)

	v.SetDefault("EXCHANGER_GRPC_HOST", "localhost")
	v.SetDefault("EXCHANGER_GRPC_PORT", 50051)
	v.SetDefault("EXCHANGER_GRPC_REQUEST_TIMEOUT_MS", 3000)

	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_TOPIC", "large-money-operations")
	v.SetDefault("LARGE_OPERATION_THRESHOLD_RUB_MINOR", 3000000)

	v.SetDefault("LOG_LEVEL", "info")
}
