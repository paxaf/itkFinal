package elastic

import (
	"context"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

type client interface {
	IndexExists(ctx context.Context, index string) (bool, error)
	CreateIndex(ctx context.Context, index string, mapping *types.TypeMapping) error
	NewBulk(index string) bulkRequest
}

type bulkRequest interface {
	UpdateOp(op types.UpdateOperation, doc interface{}, update *types.UpdateAction) error
	Do(ctx context.Context) (*bulk.Response, error)
}

type typedClient struct {
	client *elasticsearch.TypedClient
}

func (c *typedClient) IndexExists(ctx context.Context, index string) (bool, error) {
	return c.client.Indices.Exists(index).Do(ctx)
}

func (c *typedClient) CreateIndex(ctx context.Context, index string, mapping *types.TypeMapping) error {
	_, err := c.client.Indices.Create(index).
		Request(&create.Request{
			Mappings: mapping,
		}).
		Do(ctx)
	return err
}

func (c *typedClient) NewBulk(index string) bulkRequest {
	return c.client.Bulk().Index(index)
}
