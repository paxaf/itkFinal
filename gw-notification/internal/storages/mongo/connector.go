package mongo

import (
	"context"
	"fmt"

	"github.com/paxaf/itkFinal/gw-notification/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
	mng "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoDB struct {
	client     *mng.Client
	collection *mng.Collection
}

func New(cfg config.Mongo) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout())
	defer cancel()
	client, err := mng.Connect(options.Client().ApplyURI(cfg.ConnectionURI()))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	db := client.Database(cfg.Database)
	collection := db.Collection(cfg.Collection)

	if err = createIndexes(ctx, collection); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}

	storage := &MongoDB{
		client:     client,
		collection: collection,
	}

	return storage, nil
}

func createIndexes(ctx context.Context, collection *mng.Collection) error {
	index := mng.IndexModel{
		Keys: bson.D{
			{Key: "event_id", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := collection.Indexes().CreateOne(ctx, index)
	if err != nil {
		return fmt.Errorf("create event_id index: %w", err)
	}
	return nil
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}
