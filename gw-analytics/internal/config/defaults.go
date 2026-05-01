package config

import "github.com/spf13/viper"

const DefaultConfigPath = "config.env"

func setDefaults(v *viper.Viper) {
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_TOPIC", "wallet.operations")
	v.SetDefault("KAFKA_GROUP_ID", "gw-analytics")
	v.SetDefault("KAFKA_MIN_BYTES", 1)
	v.SetDefault("KAFKA_MAX_BYTES", 10485760)
	v.SetDefault("KAFKA_MAX_WAIT_MS", 500)
	v.SetDefault("KAFKA_BATCH_SIZE", 128)
	v.SetDefault("KAFKA_BATCH_WAIT_MS", 50)

	v.SetDefault("ELASTIC_ADDRESSES", "http://localhost:9200")
	v.SetDefault("ELASTIC_USERNAME", "")
	v.SetDefault("ELASTIC_PASSWORD", "")
	v.SetDefault("ELASTIC_INDEX", "wallet_operations")

	v.SetDefault("LOG_LEVEL", "info")
}
