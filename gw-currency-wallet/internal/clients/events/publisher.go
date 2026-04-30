package events

import "context"

type Publisher interface {
	NotifyLargeOperation(ctx context.Context, event LargeOperationEvent) error
	Close() error
}
