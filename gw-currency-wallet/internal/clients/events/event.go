package events

import "time"

type LargeOperationEvent struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	OperationType  string    `json:"operation_type"`
	Currency       string    `json:"currency"`
	AmountMinor    int64     `json:"amount_minor"`
	AmountRubMinor int64     `json:"amount_rub_minor"`
	CreatedAt      time.Time `json:"created_at"`
}
