package http

import (
	"bytes"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/auth"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/logger"
	httpmocks "github.com/paxaf/itkFinal/gw-currency-wallet/internal/mocks/transport/http"
	usecaseapimocks "github.com/paxaf/itkFinal/gw-currency-wallet/internal/mocks/usecaseapi"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/usecase"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HandlerSuite struct {
	suite.Suite

	uc     *usecaseapimocks.UseCaseMock
	tokens *httpmocks.TokenParserMock
	router *gin.Engine
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	gin.SetMode(gin.TestMode)
	s.uc = usecaseapimocks.NewUseCaseMock(s.T())
	s.tokens = httpmocks.NewTokenParserMock(s.T())
	h := NewHandler(s.uc, s.tokens, logger.New("error"))
	s.router = NewRouter(h, false)
}

func (s *HandlerSuite) TestRegisterSuccess() {
	s.uc.EXPECT().Register(mock.Anything, mock.MatchedBy(func(user domain.RegisterUser) bool {
		return user.Username == "paxaf" && user.Password == "secret1" && user.Email == "paxaf@example.com"
	})).Return(domain.User{ID: 7, Username: "paxaf", Email: "paxaf@example.com"}, nil).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/register", `{"username":"paxaf","password":"secret1","email":"paxaf@example.com"}`, "")

	s.Require().Equal(stdhttp.StatusCreated, resp.Code)
	s.Require().JSONEq(`{"message":"User registered successfully"}`, resp.Body.String())
}

func (s *HandlerSuite) TestHealth() {
	resp := s.request(stdhttp.MethodGet, "/api/v1/health", "", "")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"status":"ok"}`, resp.Body.String())
}

func (s *HandlerSuite) TestRegisterBadJSON() {
	resp := s.request(stdhttp.MethodPost, "/api/v1/register", `{"username":"paxaf",}`, "")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
}

func (s *HandlerSuite) TestRegisterDuplicateUser() {
	s.uc.EXPECT().Register(mock.Anything, mock.MatchedBy(func(user domain.RegisterUser) bool {
		return user.Username == "paxaf" && user.Password == "secret1" && user.Email == "paxaf@example.com"
	})).Return(domain.User{}, storages.ErrDuplicateUser).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/register", `{"username":"paxaf","password":"secret1","email":"paxaf@example.com"}`, "")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
	s.Require().JSONEq(`{"error":"Username or email already exists"}`, resp.Body.String())
}

func (s *HandlerSuite) TestLoginSuccess() {
	s.uc.EXPECT().Login(mock.Anything, mock.MatchedBy(func(user domain.LoginUser) bool {
		return user.Username == "paxaf" && user.Password == "secret1"
	})).Return("jwt-token", nil).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/login", `{"username":"paxaf","password":"secret1"}`, "")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"token":"jwt-token"}`, resp.Body.String())
}

func (s *HandlerSuite) TestLoginInvalidCredentials() {
	s.uc.EXPECT().Login(mock.Anything, mock.MatchedBy(func(user domain.LoginUser) bool {
		return user.Username == "paxaf" && user.Password == "badpass"
	})).Return("", domain.ErrInvalidCredentials).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/login", `{"username":"paxaf","password":"badpass"}`, "")

	s.Require().Equal(stdhttp.StatusUnauthorized, resp.Code)
}

func (s *HandlerSuite) TestBalanceRequiresToken() {
	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "")

	s.Require().Equal(stdhttp.StatusUnauthorized, resp.Code)
}

func (s *HandlerSuite) TestProtectedRouteRejectsInvalidToken() {
	s.tokens.EXPECT().Parse("broken").Return(int64(0), auth.ErrInvalidToken).Once()

	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "Bearer broken")

	s.Require().Equal(stdhttp.StatusUnauthorized, resp.Code)
}

func (s *HandlerSuite) TestGetBalanceSuccess() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().GetBalance(mock.Anything, int64(7)).Return(map[string]int64{"USD": 10050, "RUB": 0, "EUR": 9200}, nil).Once()

	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"balance":{"USD":100.5,"RUB":0,"EUR":92}}`, resp.Body.String())
}

func (s *HandlerSuite) TestGetBalanceUserNotFound() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().GetBalance(mock.Anything, int64(7)).Return(nil, storages.ErrUserNotFound).Once()

	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusNotFound, resp.Code)
}

func (s *HandlerSuite) TestDepositSuccess() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().Deposit(mock.Anything, int64(7), "USD", int64(10050)).Return(map[string]int64{"USD": 10050, "RUB": 0, "EUR": 0}, nil).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/deposit", `{"amount":100.50,"currency":"USD"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"message":"Account topped up successfully","new_balance":{"USD":100.5,"RUB":0,"EUR":0}}`, resp.Body.String())
}

func (s *HandlerSuite) TestDepositBadAmount() {
	s.expectToken("ok", 7)

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/deposit", `{"amount":100.999,"currency":"USD"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
	s.Require().JSONEq(`{"error":"Invalid amount or currency"}`, resp.Body.String())
}

func (s *HandlerSuite) TestDepositUsecaseError() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().Deposit(mock.Anything, int64(7), "GBP", int64(10050)).Return(nil, domain.ErrInvalidCurrency).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/deposit", `{"amount":100.50,"currency":"GBP"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
	s.Require().JSONEq(`{"error":"Invalid amount or currency"}`, resp.Body.String())
}

func (s *HandlerSuite) TestWithdrawInsufficientFunds() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().Withdraw(mock.Anything, int64(7), "USD", int64(10050)).Return(nil, domain.ErrInsufficientFunds).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/withdraw", `{"amount":100.50,"currency":"USD"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
	s.Require().JSONEq(`{"error":"Insufficient funds or invalid amount"}`, resp.Body.String())
}

func (s *HandlerSuite) TestWithdrawSuccess() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().Withdraw(mock.Anything, int64(7), "USD", int64(5000)).Return(map[string]int64{"USD": 5000, "EUR": 0, "RUB": 0}, nil).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/withdraw", `{"amount":50,"currency":"USD"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"message":"Withdrawal successful","new_balance":{"USD":50,"EUR":0,"RUB":0}}`, resp.Body.String())
}

func (s *HandlerSuite) TestGetExchangeRatesSuccess() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().GetExchangeRates(mock.Anything).Return(map[string]float64{"USD": 1, "EUR": 0.92, "RUB": 90}, nil).Once()

	resp := s.request(stdhttp.MethodGet, "/api/v1/exchange/rates", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"rates":{"USD":1,"EUR":0.92,"RUB":90}}`, resp.Body.String())
}

func (s *HandlerSuite) TestGetExchangeRatesError() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().GetExchangeRates(mock.Anything).Return(nil, domain.ErrExchangeRateUnavailable).Once()

	resp := s.request(stdhttp.MethodGet, "/api/v1/exchange/rates", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusInternalServerError, resp.Code)
}

func (s *HandlerSuite) TestExchangeSuccess() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().Exchange(mock.Anything, domain.ExchangeOperation{
		UserID:       7,
		FromCurrency: domain.CurrencyUSD,
		ToCurrency:   domain.CurrencyEUR,
		AmountMinor:  10000,
	}).Return(usecase.ExchangeResult{
		ExchangedAmountMinor: 9200,
		NewBalance:           map[string]int64{"USD": 0, "EUR": 9200, "RUB": 0},
	}, nil).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/exchange", `{"from_currency":"USD","to_currency":"EUR","amount":100}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"message":"Exchange successful","exchanged_amount":92,"new_balance":{"USD":0,"EUR":92,"RUB":0}}`, resp.Body.String())
}

func (s *HandlerSuite) TestExchangeBadCurrency() {
	s.expectToken("ok", 7)

	resp := s.request(stdhttp.MethodPost, "/api/v1/exchange", `{"from_currency":"GBP","to_currency":"EUR","amount":100}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
	s.Require().JSONEq(`{"error":"Insufficient funds or invalid currencies"}`, resp.Body.String())
}

func (s *HandlerSuite) TestExchangeBadAmount() {
	s.expectToken("ok", 7)

	resp := s.request(stdhttp.MethodPost, "/api/v1/exchange", `{"from_currency":"USD","to_currency":"EUR","amount":0}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
}

func (s *HandlerSuite) TestExchangeInsufficientFunds() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().Exchange(mock.Anything, domain.ExchangeOperation{
		UserID:       7,
		FromCurrency: domain.CurrencyUSD,
		ToCurrency:   domain.CurrencyEUR,
		AmountMinor:  10000,
	}).Return(usecase.ExchangeResult{}, domain.ErrInsufficientFunds).Once()

	resp := s.request(stdhttp.MethodPost, "/api/v1/exchange", `{"from_currency":"USD","to_currency":"EUR","amount":100}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
	s.Require().JSONEq(`{"error":"Insufficient funds or invalid currencies"}`, resp.Body.String())
}

func (s *HandlerSuite) TestInternalErrorBranch() {
	s.expectToken("ok", 7)
	s.uc.EXPECT().GetBalance(mock.Anything, int64(7)).Return(nil, errors.New("db unavailable")).Once()

	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusInternalServerError, resp.Code)
}

func (s *HandlerSuite) TestAmountToMinor() {
	tests := map[string]int64{
		"100":    10000,
		"100.5":  10050,
		"100.05": 10005,
	}
	for raw, want := range tests {
		got, err := amountToMinor(json.Number(raw))
		s.Require().NoError(err)
		s.Require().Equal(want, got)
	}

	invalid := []string{"", "-1", "+1", ".10", "1.001", "abc", "1.ab"}
	for _, raw := range invalid {
		_, err := amountToMinor(json.Number(raw))
		s.Require().ErrorIs(err, domain.ErrInvalidAmount)
	}
}

func (s *HandlerSuite) expectToken(token string, userID int64) {
	s.tokens.EXPECT().Parse(token).Return(userID, nil).Once()
}

func (s *HandlerSuite) request(method string, path string, body string, authHeader string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp := httptest.NewRecorder()
	s.router.ServeHTTP(resp, req)
	return resp
}
