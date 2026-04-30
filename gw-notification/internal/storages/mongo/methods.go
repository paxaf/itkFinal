package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	mng "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type largeOperationDocument struct {
	EventID        string    `bson:"event_id"`
	UserID         int64     `bson:"user_id"`
	OperationType  string    `bson:"operation_type"`
	Currency       string    `bson:"currency"`
	AmountMinor    int64     `bson:"amount_minor"`
	AmountRubMinor int64     `bson:"amount_rub_minor"`
	CreatedAt      time.Time `bson:"created_at"`
}

func (m *MongoDB) SaveLargeOperations(ctx context.Context, events []domain.LargeOperationEvent) error {
	if len(events) == 0 {
		return nil
	}

	docs := make([]largeOperationDocument, len(events))
	for i, event := range events {
		doc := newLargeOpDocument(event)
		docs[i] = doc
	}

	_, err := m.collection.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	if err != nil {
		if mng.IsDuplicateKeyError(err) {
			return nil
		}
		return fmt.Errorf("insert large operation: %w", err)
	}
	return nil
}

func (m *MongoDB) SaveLargeOperation(ctx context.Context, event domain.LargeOperationEvent) error {
	return m.SaveLargeOperations(ctx, []domain.LargeOperationEvent{event})
}

func newLargeOpDocument(event domain.LargeOperationEvent) largeOperationDocument {
	return largeOperationDocument{
		EventID:        event.EventID,
		UserID:         event.UserID,
		OperationType:  event.OperationType,
		Currency:       event.Currency,
		AmountMinor:    event.AmountMinor,
		AmountRubMinor: event.AmountRubMinor,
		CreatedAt:      event.CreatedAt,
	}
}
