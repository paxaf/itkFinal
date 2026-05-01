package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidEventID       = errors.New("invalid event id")
	ErrInvalidUserID        = errors.New("invalid user id")
	ErrInvalidOperationType = errors.New("invalid operation type")
	ErrInvalidStatus        = errors.New("invalid status")
	ErrInvalidCurrency      = errors.New("invalid currency")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidCreatedAt     = errors.New("invalid created at")
	ErrInvalidErrorMessage  = errors.New("invalid error message")
)

var supportedOperationTypes = map[string]struct{}{
	"DEPOSIT":  {},
	"WITHDRAW": {},
	"EXCHANGE": {},
}

var supportedStatuses = map[string]struct{}{
	"SUCCESS": {},
	"FAILED":  {},
}

var supportedCurrencies = map[string]struct{}{
	"USD": {},
	"RUB": {},
	"EUR": {},
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

func (e OperationEvent) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return ErrInvalidEventID
	}
	if e.UserID <= 0 {
		return ErrInvalidUserID
	}
	operationType := strings.ToUpper(strings.TrimSpace(e.OperationType))
	if _, ok := supportedOperationTypes[operationType]; !ok {
		return ErrInvalidOperationType
	}
	status := strings.ToUpper(strings.TrimSpace(e.Status))
	if _, ok := supportedStatuses[status]; !ok {
		return ErrInvalidStatus
	}
	currency := strings.ToUpper(strings.TrimSpace(e.Currency))
	if _, ok := supportedCurrencies[currency]; !ok {
		return ErrInvalidCurrency
	}
	if e.AmountMinor <= 0 {
		return ErrInvalidAmount
	}
	if e.AmountRubMinor <= 0 {
		return ErrInvalidAmount
	}
	if e.CreatedAt.IsZero() {
		return ErrInvalidCreatedAt
	}
	if status == "FAILED" && strings.TrimSpace(e.Error) == "" {
		return ErrInvalidErrorMessage
	}
	return nil
}
