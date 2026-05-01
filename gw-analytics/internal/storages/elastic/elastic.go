package elastic

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/paxaf/itkFinal/gw-analytics/internal/config"
)

type Storage struct {
	client client
	index  string
}

func New(ctx context.Context, cfg config.Elasticsearch) (*Storage, error) {
	esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: cfg.AddressList(),
		Username:  cfg.Username,
		Password:  cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("init elastic client: %w", err)
	}
	storage := &Storage{
		client: &typedClient{client: esClient},
		index:  cfg.Index,
	}

	err = storage.ensureIndex(ctx)

	if err != nil {
		return nil, fmt.Errorf("ensure elastic index: %w", err)
	}

	return storage, nil
}

func (s *Storage) ensureIndex(ctx context.Context) error {
	exists, err := s.client.IndexExists(ctx, s.index)
	if err != nil {
		return fmt.Errorf("check elastic index exists: %w", err)
	}
	if exists {
		return nil
	}

	return s.createIndex(ctx)
}

func (s *Storage) createIndex(ctx context.Context) error {
	if err := s.client.CreateIndex(ctx, s.index, operationEventMapping()); err != nil {
		return fmt.Errorf("create elastic index: %w", err)
	}

	return nil
}

func operationEventMapping() *types.TypeMapping {
	return &types.TypeMapping{
		Properties: map[string]types.Property{
			"event_id":         types.KeywordProperty{},
			"user_id":          types.LongNumberProperty{},
			"operation_type":   types.KeywordProperty{},
			"status":           types.KeywordProperty{},
			"currency":         types.KeywordProperty{},
			"amount_minor":     types.LongNumberProperty{},
			"amount_rub_minor": types.LongNumberProperty{},
			"created_at":       types.DateProperty{},
			"delivered_at":     types.DateProperty{},
			"latency_ms":       types.LongNumberProperty{},
			"delivery_count":   types.IntegerNumberProperty{},
			"duplicate_count":  types.IntegerNumberProperty{},
			"error":            types.TextProperty{},
		},
	}
}
