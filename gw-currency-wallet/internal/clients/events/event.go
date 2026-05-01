package events

import (
	"time"
)

const (
	OperationTypeDeposit  = "DEPOSIT"
	OperationTypeWithdraw = "WITHDRAW"
	OperationTypeExchange = "EXCHANGE"
)

const (
	OperationStatusSuccess = "SUCCESS"
	OperationStatusFailed  = "FAILED"
)

type LargeOperationEvent struct {
	CreatedAt      time.Time `json:"created_at"`
	EventID        string    `json:"event_id"`
	OperationType  string    `json:"operation_type"`
	Currency       string    `json:"currency"`
	UserID         int64     `json:"user_id"`
	AmountMinor    int64     `json:"amount_minor"`
	AmountRubMinor int64     `json:"amount_rub_minor"`
}

type OperationEvent struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	OperationType  string    `json:"operation_type"`
	Status         string    `json:"status"`
	Currency       string    `json:"currency"`
	AmountMinor    int64     `json:"amount_minor"`
	AmountRubMinor int64     `json:"amount_rub_minor"`
	CreatedAt      time.Time `json:"created_at"`
	Error          string    `json:"error,omitempty"`
}
