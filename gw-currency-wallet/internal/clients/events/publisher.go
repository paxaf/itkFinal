package events

import "context"

type PublishLargeOperation interface {
	NotifyLargeOperation(ctx context.Context, event LargeOperationEvent) error
	Close() error
}

type PublishOperation interface{}
