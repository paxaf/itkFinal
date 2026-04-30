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
	ErrInvalidCurrency      = errors.New("invalid currency")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidCreatedAt     = errors.New("invalid created at")
)

var supportedOperationTypes = map[string]struct{}{
	"DEPOSIT":  {},
	"WITHDRAW": {},
	"EXCHANGE": {},
}

var supportedCurrencies = map[string]struct{}{
	"USD": {},
	"RUB": {},
	"EUR": {},
}

type LargeOperationEvent struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	OperationType  string    `json:"operation_type"`
	Currency       string    `json:"currency"`
	AmountMinor    int64     `json:"amount_minor"`
	AmountRubMinor int64     `json:"amount_rub_minor"`
	CreatedAt      time.Time `json:"created_at"`
}

func (e LargeOperationEvent) Validate() error {
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
	return nil
}
