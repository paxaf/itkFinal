package events

import (
	"context"
	"fmt"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/config"
	"github.com/segmentio/kafka-go"
)

type KafkaNotifier struct {
	writer *kafka.Writer
}

func New(ctx context.Context, cfg config.Kafka) (*KafkaNotifier, error) {
	if err := pingKafka(ctx, cfg.BrokerList()); err != nil {
		return nil, fmt.Errorf("ping kafka: %w", err)
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.BrokerList()...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
	}

	return &KafkaNotifier{
		writer: writer,
	}, nil
}

func pingKafka(ctx context.Context, brokers []string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("empty kafka brokers")
	}

	dialer := &kafka.Dialer{}

	var lastErr error

	for _, broker := range brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("all kafka brokers unavailable: %w", lastErr)
}
