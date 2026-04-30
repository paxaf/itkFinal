package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/config"
	"github.com/segmentio/kafka-go"
)

type KafkaPublisher struct {
	writer               *kafka.Writer
	operationsTopic      string
	largeOperationsTopic string
}

func New(ctx context.Context, cfg config.Kafka) (*KafkaPublisher, error) {
	if err := pingKafka(ctx, cfg.BrokerList()); err != nil {
		return nil, fmt.Errorf("ping kafka: %w", err)
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.BrokerList()...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
	}

	return &KafkaPublisher{
		writer:               writer,
		operationsTopic:      cfg.OperationsTopic,
		largeOperationsTopic: cfg.LargeOperationsTopic,
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

func (k *KafkaPublisher) Close() error {
	if k.writer == nil {
		return nil
	}
	err := k.writer.Close()
	if err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}

func (k *KafkaPublisher) PublishLargeOperation(ctx context.Context, event LargeOperationEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal large op event: %w", err)
	}

	if err = k.writer.WriteMessages(ctx, kafka.Message{
		Topic: k.largeOperationsTopic,
		Key:   []byte(strconv.FormatInt(event.UserID, 10)),
		Value: payload,
	}); err != nil {
		return fmt.Errorf("write large operation event: %w", err)
	}
	return nil
}
