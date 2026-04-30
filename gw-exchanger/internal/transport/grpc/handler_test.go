package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/paxaf/itkFinal/gw-exchanger/internal/logger"
	usecasemocks "github.com/paxaf/itkFinal/gw-exchanger/internal/mocks/usecase"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/usecase"
	exchangegrpc "github.com/paxaf/itkFinal/proto-exchange/exchange"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HandlerSuite struct {
	suite.Suite

	ctx       context.Context
	exchanger *usecasemocks.ExchangerMock
	handler   *Handler
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	s.ctx = context.Background()
	s.exchanger = usecasemocks.NewExchangerMock(s.T())
	s.handler = NewHandler(s.exchanger, logger.New("error"))
}

func (s *HandlerSuite) TestGetExchangeRatesSuccess() {
	s.exchanger.EXPECT().
		GetRates(mock.Anything).
		Return(map[string]float64{
			"USD": 1,
			"EUR": 0.92,
			"RUB": 90,
		}, nil)

	resp, err := s.handler.GetExchangeRates(s.ctx, &exchangegrpc.Empty{})

	s.Require().NoError(err)
	s.Require().Equal(map[string]float32{
		"USD": 1,
		"EUR": float32(0.92),
		"RUB": 90,
	}, resp.GetRates())
}

func (s *HandlerSuite) TestGetExchangeRatesReturnsInternalOnUsecaseError() {
	s.exchanger.EXPECT().
		GetRates(mock.Anything).
		Return(nil, errors.New("storage unavailable"))

	resp, err := s.handler.GetExchangeRates(s.ctx, &exchangegrpc.Empty{})

	s.Require().Nil(resp)
	s.Require().Equal(codes.Internal, status.Code(err))
}

func (s *HandlerSuite) TestGetExchangeRateForCurrencySuccess() {
	s.exchanger.EXPECT().
		GetRate(mock.Anything, " usd ", "eur").
		Return(0.92, nil)

	resp, err := s.handler.GetExchangeRateForCurrency(s.ctx, &exchangegrpc.CurrencyRequest{
		FromCurrency: " usd ",
		ToCurrency:   "eur",
	})

	s.Require().NoError(err)
	s.Require().Equal("USD", resp.GetFromCurrency())
	s.Require().Equal("EUR", resp.GetToCurrency())
	s.Require().Equal(float32(0.92), resp.GetExchangeRate())
}

func (s *HandlerSuite) TestGetExchangeRateForCurrencyReturnsInvalidArgumentOnNilRequest() {
	resp, err := s.handler.GetExchangeRateForCurrency(s.ctx, nil)

	s.Require().Nil(resp)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
}

func (s *HandlerSuite) TestGetExchangeRateForCurrencyReturnsInvalidArgumentOnCurrencyError() {
	s.exchanger.EXPECT().
		GetRate(mock.Anything, "US", "EUR").
		Return(0, usecase.ErrInvalidCurrency)

	resp, err := s.handler.GetExchangeRateForCurrency(s.ctx, &exchangegrpc.CurrencyRequest{
		FromCurrency: "US",
		ToCurrency:   "EUR",
	})

	s.Require().Nil(resp)
	s.Require().Equal(codes.InvalidArgument, status.Code(err))
}

func (s *HandlerSuite) TestGetExchangeRateForCurrencyReturnsInternalOnUsecaseError() {
	s.exchanger.EXPECT().
		GetRate(mock.Anything, "USD", "EUR").
		Return(0, errors.New("storage unavailable"))

	resp, err := s.handler.GetExchangeRateForCurrency(s.ctx, &exchangegrpc.CurrencyRequest{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
	})

	s.Require().Nil(resp)
	s.Require().Equal(codes.Internal, status.Code(err))
}
