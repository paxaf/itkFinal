package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/config"
	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	"github.com/paxaf/itkFinal/gw-notification/internal/logger"
	"github.com/paxaf/itkFinal/gw-notification/internal/usecase"
	kafkago "github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader    reader
	handler   usecase.LargeOperationHandler
	log       logger.Interface
	batchSize int
	batchWait time.Duration
}

type reader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
	Close() error
}

func New(cfg config.Kafka, handler usecase.LargeOperationHandler, log logger.Interface) *Consumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  cfg.BrokerList(),
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: cfg.MinBytes,
		MaxBytes: cfg.MaxBytes,
		MaxWait:  cfg.MaxWait(),
	})
	return &Consumer{
		reader:    reader,
		handler:   handler,
		log:       log,
		batchSize: cfg.BatchSize,
		batchWait: cfg.BatchWait(),
	}
}

func (c *Consumer) Close() error {
	if c.reader == nil {
		return nil
	}

	return c.reader.Close()
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		messages, err := c.fetchBatch(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			c.reportError("fetch kafka batch failed", err)
			continue
		}

		if err = c.handleMessages(ctx, messages); err != nil {
			return fmt.Errorf("handle kafka batch: %w", err)
		}

		if err = c.reader.CommitMessages(ctx, messages...); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			c.reportError("commit kafka batch failed", err)
			continue
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	return c.handleMessages(ctx, []kafkago.Message{msg})
}

func (c *Consumer) handleMessages(ctx context.Context, messages []kafkago.Message) error {
	events := make([]domain.LargeOperationEvent, 0, len(messages))
	decodeFailed := 0

	for _, msg := range messages {
		var event domain.LargeOperationEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			decodeFailed++
			c.reportWarn("decode kafka message failed", err, map[string]interface{}{
				"topic":     msg.Topic,
				"partition": msg.Partition,
				"offset":    msg.Offset,
			})
			continue
		}

		events = append(events, event)
	}

	if len(events) == 0 {
		c.logInfo("large operation batch skipped", map[string]interface{}{
			"messages":      len(messages),
			"decoded":       0,
			"decode_failed": decodeFailed,
		})
		return nil
	}

	result, err := c.handler.HandleLargeOperations(ctx, events)
	if err != nil {
		return fmt.Errorf("handle large operation events: %w", err)
	}

	c.logInfo("large operation batch handled", map[string]interface{}{
		"messages":      len(messages),
		"decoded":       len(events),
		"decode_failed": decodeFailed,
		"received":      result.Received,
		"valid":         result.Valid,
		"invalid":       result.Invalid,
		"accepted":      result.Accepted,
	})

	return nil
}

func (c *Consumer) fetchBatch(ctx context.Context) ([]kafkago.Message, error) {
	first, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch first message: %w", err)
	}
	messages := make([]kafkago.Message, 0, c.batchSize)
	messages = append(messages, first)

	if c.batchSize <= 1 {
		return messages, nil
	}

	batchCtx, cancel := context.WithTimeout(ctx, c.batchWait)
	defer cancel()

	for len(messages) < c.batchSize {
		msg, err := c.reader.FetchMessage(batchCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || (errors.Is(err, context.Canceled) && ctx.Err() == nil) {
				break
			}
			if errors.Is(err, context.Canceled) {
				return nil, err
			}
			c.reportError("fetch kafka batch message failed", err)
			break
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (c *Consumer) reportError(message string, err error) {
	if c.log == nil || err == nil {
		return
	}

	c.log.Error(message, map[string]interface{}{
		"error": err.Error(),
	})
}

func (c *Consumer) reportWarn(message string, err error, fields map[string]interface{}) {
	if c.log == nil {
		return
	}

	if fields == nil {
		fields = make(map[string]interface{})
	}
	if err != nil {
		fields["error"] = err.Error()
	}

	c.log.Warn(message, fields)
}

func (c *Consumer) logInfo(message string, fields map[string]interface{}) {
	if c.log == nil {
		return
	}

	c.log.Info(message, fields)
}
