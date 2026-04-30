package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"
)

type HelpersSuite struct {
	suite.Suite
}

func TestHelpersSuite(t *testing.T) {
	suite.Run(t, new(HelpersSuite))
}

func (s *HelpersSuite) TestPgErrorHelpers() {
	s.Require().True(isUniqueViolation(&pgconn.PgError{Code: "23505"}))
	s.Require().False(isUniqueViolation(&pgconn.PgError{Code: "23503"}))
}
