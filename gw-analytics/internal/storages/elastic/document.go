package elastic

import (
	"time"

	"github.com/paxaf/itkFinal/gw-analytics/internal/domain"
)

type operationDocument struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	OperationType  string    `json:"operation_type"`
	Status         string    `json:"status"`
	Currency       string    `json:"currency"`
	AmountMinor    int64     `json:"amount_minor"`
	AmountRubMinor int64     `json:"amount_rub_minor"`
	CreatedAt      time.Time `json:"created_at"`
	DeliveredAt    time.Time `json:"delivered_at"`
	LatencyMS      int64     `json:"latency_ms"`
	DeliveryCount  int       `json:"delivery_count"`
	DuplicateCount int       `json:"duplicate_count"`
	Error          string    `json:"error,omitempty"`
}

func newOperationDocument(event domain.OperationEvent, deliveredAt time.Time) operationDocument {
	latency := deliveredAt.Sub(event.CreatedAt).Milliseconds()
	if latency < 0 {
		latency = 0
	}

	return operationDocument{
		EventID:        event.EventID,
		UserID:         event.UserID,
		OperationType:  event.OperationType,
		Status:         event.Status,
		Currency:       event.Currency,
		AmountMinor:    event.AmountMinor,
		AmountRubMinor: event.AmountRubMinor,
		CreatedAt:      event.CreatedAt,
		DeliveredAt:    deliveredAt,
		LatencyMS:      latency,
		DeliveryCount:  1,
		DuplicateCount: 0,
		Error:          event.Error,
	}
}
