package mongo

import (
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-notification/internal/domain"
	"github.com/stretchr/testify/suite"
)

type MethodsSuite struct {
	suite.Suite
}

func TestMethodsSuite(t *testing.T) {
	suite.Run(t, new(MethodsSuite))
}

func (s *MethodsSuite) TestNewLargeOperationDocument() {
	event := domain.LargeOperationEvent{
		EventID:        "event-1",
		UserID:         42,
		OperationType:  "WITHDRAW",
		Currency:       "USD",
		AmountMinor:    100_00,
		AmountRubMinor: 9_000_00,
		CreatedAt:      time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	}

	doc := newLargeOpDocument(event)

	s.Require().Equal(event.EventID, doc.EventID)
	s.Require().Equal(event.UserID, doc.UserID)
	s.Require().Equal(event.OperationType, doc.OperationType)
	s.Require().Equal(event.Currency, doc.Currency)
	s.Require().Equal(event.AmountMinor, doc.AmountMinor)
	s.Require().Equal(event.AmountRubMinor, doc.AmountRubMinor)
	s.Require().Equal(event.CreatedAt, doc.CreatedAt)
}
