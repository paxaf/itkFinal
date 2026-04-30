package events

import "context"

type LargeOperationPublisher interface {
	PublishLargeOperation(ctx context.Context, event LargeOperationEvent) error
	Close() error
}
