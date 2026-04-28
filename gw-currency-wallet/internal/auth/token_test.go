package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TokenSuite struct {
	suite.Suite

	manager *Manager
}

func TestTokenSuite(t *testing.T) {
	suite.Run(t, new(TokenSuite))
}

func (s *TokenSuite) SetupTest() {
	s.manager = NewManager("secret", time.Hour)
}

func (s *TokenSuite) TestGenerateAndParse() {
	token, err := s.manager.Generate(42)
	s.Require().NoError(err)
	s.Require().NotEmpty(token)

	userID, err := s.manager.Parse(token)
	s.Require().NoError(err)
	s.Require().Equal(int64(42), userID)
}

func (s *TokenSuite) TestParseInvalidToken() {
	userID, err := s.manager.Parse("broken")

	s.Require().Zero(userID)
	s.Require().ErrorIs(err, ErrInvalidToken)
}
