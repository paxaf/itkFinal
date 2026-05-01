package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
)

const updateDeliveryStatsScript = `
ctx._source.delivery_count = ctx._source.delivery_count + 1;
ctx._source.duplicate_count = ctx._source.duplicate_count + 1;
`

func (s *Storage) SaveOperations(ctx context.Context, events []domain.OperationEvent) error {
	if len(events) == 0 {
		return nil
	}
	deliveredAt := time.Now().UTC()

	bulkRequest := s.client.NewBulk(s.index)

	for _, event := range events {
		doc := newOperationDocument(event, deliveredAt)

		action, err := newOperationUpdateAction(doc)
		if err != nil {
			return err
		}

		err = bulkRequest.UpdateOp(types.UpdateOperation{
			Id_: &doc.EventID,
		}, nil, action)
		if err != nil {
			return fmt.Errorf("add operation event to bulk: %w", err)
		}
	}
	resp, err := bulkRequest.Do(ctx)
	if err != nil {
		return fmt.Errorf("save operation events bulk: %w", err)
	}
	if resp.Errors {
		return errors.New("save operation events bulk: elastic returned item errors")
	}
	return nil
}

func (s *Storage) SaveOperation(ctx context.Context, event domain.OperationEvent) error {
	return s.SaveOperations(ctx, []domain.OperationEvent{event})
}

func (s *Storage) Close(_ context.Context) error {
	return nil
}

func newOperationUpdateAction(doc operationDocument) (*types.UpdateAction, error) {
	upsert, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal operation document: %w", err)
	}

	return &types.UpdateAction{
		Script: &types.Script{
			Source: updateDeliveryStatsScript,
		},
		Upsert: upsert,
	}, nil
}
