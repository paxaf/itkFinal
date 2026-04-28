package http

import (
	"bytes"
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/logger"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/usecase"
	"github.com/stretchr/testify/suite"
)

type HandlerSuite struct {
	suite.Suite

	uc     *fakeUseCase
	tokens *fakeTokenParser
	router *gin.Engine
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	gin.SetMode(gin.TestMode)
	s.uc = &fakeUseCase{}
	s.tokens = &fakeTokenParser{userID: 7}
	h := NewHandler(s.uc, s.tokens, logger.New("error"))
	s.router = NewRouter(h, false)
}

func (s *HandlerSuite) TestRegisterSuccess() {
	s.uc.registerFn = func(ctx context.Context, user domain.RegisterUser) (domain.User, error) {
		s.Require().Equal("paxaf", user.Username)
		return domain.User{ID: 7, Username: user.Username, Email: user.Email}, nil
	}

	resp := s.request(stdhttp.MethodPost, "/api/v1/register", `{"username":"paxaf","password":"secret1","email":"paxaf@example.com"}`, "")

	s.Require().Equal(stdhttp.StatusCreated, resp.Code)
	s.Require().JSONEq(`{"message":"User registered successfully"}`, resp.Body.String())
}

func (s *HandlerSuite) TestLoginSuccess() {
	s.uc.loginFn = func(ctx context.Context, user domain.LoginUser) (string, error) {
		s.Require().Equal("paxaf", user.Username)
		return "jwt-token", nil
	}

	resp := s.request(stdhttp.MethodPost, "/api/v1/login", `{"username":"paxaf","password":"secret1"}`, "")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"token":"jwt-token"}`, resp.Body.String())
}

func (s *HandlerSuite) TestBalanceRequiresToken() {
	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "")

	s.Require().Equal(stdhttp.StatusUnauthorized, resp.Code)
}

func (s *HandlerSuite) TestGetBalanceSuccess() {
	s.uc.balanceFn = func(ctx context.Context, userID int64) (map[string]int64, error) {
		s.Require().Equal(int64(7), userID)
		return map[string]int64{"USD": 10050, "RUB": 0, "EUR": 9200}, nil
	}

	resp := s.request(stdhttp.MethodGet, "/api/v1/balance", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"balance":{"USD":100.5,"RUB":0,"EUR":92}}`, resp.Body.String())
}

func (s *HandlerSuite) TestDepositSuccess() {
	s.uc.depositFn = func(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error) {
		s.Require().Equal(int64(7), userID)
		s.Require().Equal("USD", currency)
		s.Require().Equal(int64(10050), amountMinor)
		return map[string]int64{"USD": 10050, "RUB": 0, "EUR": 0}, nil
	}

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/deposit", `{"amount":100.50,"currency":"USD"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"message":"Account topped up successfully","new_balance":{"USD":100.5,"RUB":0,"EUR":0}}`, resp.Body.String())
}

func (s *HandlerSuite) TestWithdrawInsufficientFunds() {
	s.uc.withdrawFn = func(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error) {
		return nil, domain.ErrInsufficientFunds
	}

	resp := s.request(stdhttp.MethodPost, "/api/v1/wallet/withdraw", `{"amount":100.50,"currency":"USD"}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusBadRequest, resp.Code)
}

func (s *HandlerSuite) TestGetExchangeRatesSuccess() {
	s.uc.ratesFn = func(ctx context.Context) (map[string]float64, error) {
		return map[string]float64{"USD": 1, "EUR": 0.92, "RUB": 90}, nil
	}

	resp := s.request(stdhttp.MethodGet, "/api/v1/exchange/rates", "", "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"rates":{"USD":1,"EUR":0.92,"RUB":90}}`, resp.Body.String())
}

func (s *HandlerSuite) TestExchangeSuccess() {
	s.uc.exchangeFn = func(ctx context.Context, op domain.ExchangeOperation) (usecase.ExchangeResult, error) {
		s.Require().Equal(int64(7), op.UserID)
		s.Require().Equal(domain.CurrencyUSD, op.FromCurrency)
		s.Require().Equal(domain.CurrencyEUR, op.ToCurrency)
		s.Require().Equal(int64(10000), op.AmountMinor)
		return usecase.ExchangeResult{
			ExchangedAmountMinor: 9200,
			NewBalance:           map[string]int64{"USD": 0, "EUR": 9200, "RUB": 0},
		}, nil
	}

	resp := s.request(stdhttp.MethodPost, "/api/v1/exchange", `{"from_currency":"USD","to_currency":"EUR","amount":100}`, "Bearer ok")

	s.Require().Equal(stdhttp.StatusOK, resp.Code)
	s.Require().JSONEq(`{"message":"Exchange successful","exchanged_amount":92,"new_balance":{"USD":0,"EUR":92,"RUB":0}}`, resp.Body.String())
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

type fakeTokenParser struct {
	userID int64
	err    error
}

func (f *fakeTokenParser) Parse(tokenValue string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.userID, nil
}

type fakeUseCase struct {
	registerFn func(ctx context.Context, user domain.RegisterUser) (domain.User, error)
	loginFn    func(ctx context.Context, user domain.LoginUser) (string, error)
	balanceFn  func(ctx context.Context, userID int64) (map[string]int64, error)
	depositFn  func(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error)
	withdrawFn func(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error)
	ratesFn    func(ctx context.Context) (map[string]float64, error)
	exchangeFn func(ctx context.Context, op domain.ExchangeOperation) (usecase.ExchangeResult, error)
}

func (f *fakeUseCase) Register(ctx context.Context, user domain.RegisterUser) (domain.User, error) {
	if f.registerFn != nil {
		return f.registerFn(ctx, user)
	}
	return domain.User{ID: 1}, nil
}

func (f *fakeUseCase) Login(ctx context.Context, user domain.LoginUser) (string, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, user)
	}
	return "token", nil
}

func (f *fakeUseCase) WalletOperation(ctx context.Context, op domain.WalletOperation) error {
	return nil
}

func (f *fakeUseCase) Deposit(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error) {
	if f.depositFn != nil {
		return f.depositFn(ctx, userID, currency, amountMinor)
	}
	return nil, nil
}

func (f *fakeUseCase) Withdraw(ctx context.Context, userID int64, currency string, amountMinor int64) (map[string]int64, error) {
	if f.withdrawFn != nil {
		return f.withdrawFn(ctx, userID, currency, amountMinor)
	}
	return nil, nil
}

func (f *fakeUseCase) GetBalance(ctx context.Context, userID int64) (map[string]int64, error) {
	if f.balanceFn != nil {
		return f.balanceFn(ctx, userID)
	}
	return nil, nil
}

func (f *fakeUseCase) GetExchangeRates(ctx context.Context) (map[string]float64, error) {
	if f.ratesFn != nil {
		return f.ratesFn(ctx)
	}
	return nil, nil
}

func (f *fakeUseCase) Exchange(ctx context.Context, op domain.ExchangeOperation) (usecase.ExchangeResult, error) {
	if f.exchangeFn != nil {
		return f.exchangeFn(ctx, op)
	}
	return usecase.ExchangeResult{}, nil
}
