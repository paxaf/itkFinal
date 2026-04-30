package config

import "github.com/spf13/viper"

const DefaultConfigPath = "config.env"

func setDefaults(v *viper.Viper) {
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_TOPIC", "large-money-operations")
	v.SetDefault("KAFKA_GROUP_ID", "gw-notification")
	v.SetDefault("KAFKA_MIN_BYTES", 1)
	v.SetDefault("KAFKA_MAX_BYTES", 10485760)
	v.SetDefault("KAFKA_MAX_WAIT_MS", 500)
	v.SetDefault("KAFKA_BATCH_SIZE", 128)
	v.SetDefault("KAFKA_BATCH_WAIT_MS", 50)

	v.SetDefault("MONGO_URI", "")
	v.SetDefault("MONGO_HOST", "localhost")
	v.SetDefault("MONGO_PORT", 27017)
	v.SetDefault("MONGO_USER", "mongo")
	v.SetDefault("MONGO_PASSWORD", "mongo")
	v.SetDefault("MONGO_AUTH_SOURCE", "admin")
	v.SetDefault("MONGO_DB", "notification")
	v.SetDefault("MONGO_COLLECTION", "large_operations")
	v.SetDefault("MONGO_CONNECT_TIMEOUT_MS", 5000)

	v.SetDefault("LOG_LEVEL", "info")
}
