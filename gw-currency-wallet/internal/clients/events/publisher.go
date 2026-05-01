package events

import "context"

type LargeOperationPublisher interface {
	PublishLargeOperation(ctx context.Context, event LargeOperationEvent) error
	Close() error
}

type OperationPublisher interface {
	PublishOperation(ctx context.Context, event OperationEvent) error
	Close() error
}
