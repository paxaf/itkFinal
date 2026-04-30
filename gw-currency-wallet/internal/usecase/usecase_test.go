package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/clients/events"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	storagesmocks "github.com/paxaf/itkFinal/gw-currency-wallet/internal/mocks/storages"
	usecasemocks "github.com/paxaf/itkFinal/gw-currency-wallet/internal/mocks/usecase"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
)

type UseCaseSuite struct {
	suite.Suite

	ctx       context.Context
	storage   *storagesmocks.StorageMock
	tokens    *usecasemocks.TokenManagerMock
	exchanger *usecasemocks.ExchangeProviderMock
	publisher *usecasemocks.LargeOperationPublisherMock
	log       *usecasemocks.LoggerMock
	service   *Service
}

func TestUseCaseSuite(t *testing.T) {
	suite.Run(t, new(UseCaseSuite))
}

func (s *UseCaseSuite) SetupTest() {
	s.ctx = context.Background()
	s.storage = storagesmocks.NewStorageMock(s.T())
	s.tokens = usecasemocks.NewTokenManagerMock(s.T())
	s.exchanger = usecasemocks.NewExchangeProviderMock(s.T())
	s.publisher = usecasemocks.NewLargeOperationPublisherMock(s.T())
	s.log = usecasemocks.NewLoggerMock(s.T())
	s.service = New(s.storage, s.tokens, s.exchanger, nil, 3000000, nil)
}

func (s *UseCaseSuite) TestRegisterHashesPasswordAndCreatesUser() {
	s.storage.EXPECT().CreateUser(
		s.ctx,
		"paxaf",
		"paxaf@example.com",
		mock.MatchedBy(func(passwordHash string) bool {
			return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("secret1")) == nil
		}),
	).Return(domain.User{ID: 1, Username: "paxaf", Email: "paxaf@example.com"}, nil).Once()

	user, err := s.service.Register(s.ctx, domain.RegisterUser{
		Username: " paxaf ",
		Email:    " paxaf@example.com ",
		Password: "secret1",
	})

	s.Require().NoError(err)
	s.Require().Equal(int64(1), user.ID)
}

func (s *UseCaseSuite) TestRegisterReturnsDuplicateUser() {
	s.storage.EXPECT().CreateUser(
		s.ctx,
		"paxaf",
		"paxaf@example.com",
		mock.AnythingOfType("string"),
	).Return(domain.User{}, storages.ErrDuplicateUser).Once()

	_, err := s.service.Register(s.ctx, domain.RegisterUser{Username: "paxaf", Email: "paxaf@example.com", Password: "secret1"})

	s.Require().ErrorIs(err, storages.ErrDuplicateUser)
}

func (s *UseCaseSuite) TestRegisterReturnsValidationError() {
	_, err := s.service.Register(s.ctx, domain.RegisterUser{Username: "", Email: "bad", Password: "123"})

	s.Require().ErrorIs(err, domain.ErrInvalidUsername)
}

func (s *UseCaseSuite) TestLoginReturnsToken() {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret1"), bcrypt.DefaultCost)
	s.Require().NoError(err)

	s.storage.EXPECT().GetUserCredentialsByUsername(s.ctx, "paxaf").Return(domain.UserCredentials{
		User:         domain.User{ID: 42, Username: "paxaf", Email: "paxaf@example.com"},
		PasswordHash: string(hash),
	}, nil).Once()
	s.tokens.EXPECT().Generate(int64(42)).Return("token", nil).Once()

	token, err := s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().NoError(err)
	s.Require().Equal("token", token)
}

func (s *UseCaseSuite) TestLoginReturnsStorageError() {
	s.storage.EXPECT().GetUserCredentialsByUsername(s.ctx, "paxaf").Return(domain.UserCredentials{}, errBoom).Once()

	_, err := s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().ErrorIs(err, errBoom)
}

func (s *UseCaseSuite) TestLoginReturnsTokenError() {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret1"), bcrypt.DefaultCost)
	s.Require().NoError(err)

	s.storage.EXPECT().GetUserCredentialsByUsername(s.ctx, "paxaf").Return(domain.UserCredentials{
		User:         domain.User{ID: 42},
		PasswordHash: string(hash),
	}, nil).Once()
	s.tokens.EXPECT().Generate(int64(42)).Return("", errBoom).Once()

	_, err = s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().ErrorIs(err, errBoom)
}

func (s *UseCaseSuite) TestLoginReturnsInvalidCredentialsOnWrongPassword() {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret1"), bcrypt.DefaultCost)
	s.Require().NoError(err)

	s.storage.EXPECT().GetUserCredentialsByUsername(s.ctx, "paxaf").Return(domain.UserCredentials{
		User:         domain.User{ID: 42},
		PasswordHash: string(hash),
	}, nil).Once()

	_, err = s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "badpass"})

	s.Require().ErrorIs(err, domain.ErrInvalidCredentials)
}

func (s *UseCaseSuite) TestLoginReturnsInvalidCredentialsWhenUserMissing() {
	s.storage.EXPECT().GetUserCredentialsByUsername(s.ctx, "paxaf").Return(domain.UserCredentials{}, storages.ErrUserNotFound).Once()

	_, err := s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().ErrorIs(err, domain.ErrInvalidCredentials)
}

func (s *UseCaseSuite) TestGetBalanceNormalizesMissingCurrencies() {
	s.storage.EXPECT().GetBalances(s.ctx, int64(1)).Return(map[string]int64{"USD": 10000}, nil).Once()

	balance, err := s.service.GetBalance(s.ctx, 1)

	s.Require().NoError(err)
	s.Require().Equal(map[string]int64{"USD": 10000, "RUB": 0, "EUR": 0}, balance)
}

func (s *UseCaseSuite) TestGetBalanceErrors() {
	_, err := s.service.GetBalance(s.ctx, 0)
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	s.storage.EXPECT().GetBalances(s.ctx, int64(1)).Return(nil, errBoom).Once()
	_, err = s.service.GetBalance(s.ctx, 1)
	s.Require().ErrorIs(err, errBoom)

	s.storage.EXPECT().GetBalances(s.ctx, int64(1)).Return(map[string]int64{"GBP": 100}, nil).Once()
	_, err = s.service.GetBalance(s.ctx, 1)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

func (s *UseCaseSuite) TestDepositCallsStorage() {
	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyUSD, int64(10050)).Return(map[string]int64{
		"USD": 10050,
		"RUB": 0,
		"EUR": 0,
	}, nil).Once()

	balance, err := s.service.Deposit(s.ctx, 1, "usd", 10050)

	s.Require().NoError(err)
	s.Require().Equal(int64(10050), balance["USD"])
}

func (s *UseCaseSuite) TestDepositPublishesLargeOperation() {
	s.service.publisher = s.publisher
	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyRUB, int64(3000000)).Return(map[string]int64{"RUB": 3000000}, nil).Once()
	s.publisher.EXPECT().PublishLargeOperation(s.ctx, mock.MatchedBy(func(event events.LargeOperationEvent) bool {
		return event.EventID != "" &&
			event.UserID == 1 &&
			event.OperationType == events.OperationTypeDeposit &&
			event.Currency == "RUB" &&
			event.AmountMinor == 3000000 &&
			event.AmountRubMinor == 3000000 &&
			!event.CreatedAt.IsZero()
	})).Return(nil).Once()

	balance, err := s.service.Deposit(s.ctx, 1, "rub", 3000000)

	s.Require().NoError(err)
	s.Require().Equal(int64(3000000), balance["RUB"])
}

func (s *UseCaseSuite) TestDepositPublishesLargeOperationWithRubEquivalent() {
	s.service.publisher = s.publisher
	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyUSD, int64(40000)).Return(map[string]int64{"USD": 40000}, nil).Once()
	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "RUB").Return(float64(90), nil).Once()
	s.publisher.EXPECT().PublishLargeOperation(s.ctx, mock.MatchedBy(func(event events.LargeOperationEvent) bool {
		return event.OperationType == events.OperationTypeDeposit &&
			event.Currency == "USD" &&
			event.AmountMinor == 40000 &&
			event.AmountRubMinor == 3600000
	})).Return(nil).Once()

	balance, err := s.service.Deposit(s.ctx, 1, "usd", 40000)

	s.Require().NoError(err)
	s.Require().Equal(int64(40000), balance["USD"])
}

func (s *UseCaseSuite) TestDepositSkipsSmallOperation() {
	s.service.publisher = s.publisher
	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyRUB, int64(2999999)).Return(map[string]int64{"RUB": 2999999}, nil).Once()

	_, err := s.service.Deposit(s.ctx, 1, "rub", 2999999)

	s.Require().NoError(err)
}

func (s *UseCaseSuite) TestDepositIgnoresPublishError() {
	s.service.publisher = s.publisher
	s.service.log = s.log
	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyRUB, int64(3000000)).Return(map[string]int64{"RUB": 3000000}, nil).Once()
	s.publisher.EXPECT().PublishLargeOperation(s.ctx, mock.AnythingOfType("events.LargeOperationEvent")).Return(errBoom).Once()
	s.log.EXPECT().Error("publish large operation failed", mock.MatchedBy(func(fields map[string]interface{}) bool {
		return fields["error"] == errBoom.Error() &&
			fields["event_id"] != "" &&
			fields["user_id"] == int64(1) &&
			fields["operation_type"] == events.OperationTypeDeposit &&
			fields["currency"] == "RUB" &&
			fields["amount_minor"] == int64(3000000) &&
			fields["amount_rub_minor"] == int64(3000000)
	})).Once()

	_, err := s.service.Deposit(s.ctx, 1, "rub", 3000000)

	s.Require().NoError(err)
}

func (s *UseCaseSuite) TestDepositErrors() {
	_, err := s.service.Deposit(s.ctx, 1, "GBP", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)

	_, err = s.service.Deposit(s.ctx, 0, "USD", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyUSD, int64(100)).Return(nil, errBoom).Once()
	_, err = s.service.Deposit(s.ctx, 1, "USD", 100)
	s.Require().ErrorIs(err, errBoom)

	s.storage.EXPECT().Deposit(s.ctx, int64(1), domain.CurrencyUSD, int64(100)).Return(map[string]int64{"GBP": 100}, nil).Once()
	_, err = s.service.Deposit(s.ctx, 1, "USD", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

func (s *UseCaseSuite) TestWithdrawReturnsInsufficientFunds() {
	s.storage.EXPECT().Withdraw(s.ctx, int64(1), domain.CurrencyUSD, int64(10050)).Return(nil, domain.ErrInsufficientFunds).Once()

	_, err := s.service.Withdraw(s.ctx, 1, "USD", 10050)

	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)
}

func (s *UseCaseSuite) TestWithdrawSuccessAndErrors() {
	s.storage.EXPECT().Withdraw(s.ctx, int64(1), domain.CurrencyUSD, int64(100)).Return(map[string]int64{"USD": 5000}, nil).Once()
	balance, err := s.service.Withdraw(s.ctx, 1, "USD", 100)
	s.Require().NoError(err)
	s.Require().Equal(int64(5000), balance["USD"])

	_, err = s.service.Withdraw(s.ctx, 1, "GBP", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)

	_, err = s.service.Withdraw(s.ctx, 1, "USD", 0)
	s.Require().ErrorIs(err, domain.ErrInvalidAmount)
}

func (s *UseCaseSuite) TestGetExchangeRates() {
	s.exchanger.EXPECT().GetRates(s.ctx).Return(map[string]float64{"USD": 1, "EUR": 0.92, "RUB": 90}, nil).Once()

	rates, err := s.service.GetExchangeRates(s.ctx)

	s.Require().NoError(err)
	s.Require().Equal(float64(0.92), rates["EUR"])
}

func (s *UseCaseSuite) TestGetExchangeRatesErrors() {
	s.service.exchanger = nil
	_, err := s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.service.exchanger = s.exchanger
	s.exchanger.EXPECT().GetRates(s.ctx).Return(nil, errBoom).Once()
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, errBoom)

	s.exchanger.EXPECT().GetRates(s.ctx).Return(map[string]float64{"GBP": 1}, nil).Once()
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)

	s.exchanger.EXPECT().GetRates(s.ctx).Return(map[string]float64{"USD": 0}, nil).Once()
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.exchanger.EXPECT().GetRates(s.ctx).Return(map[string]float64{"USD": 1}, nil).Once()
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)
}

func (s *UseCaseSuite) TestExchangeConvertsAndStoresAtomically() {
	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(0.92), nil).Once()
	s.storage.EXPECT().Exchange(
		s.ctx,
		int64(1),
		domain.CurrencyUSD,
		domain.CurrencyEUR,
		int64(10000),
		int64(9200),
	).Return(map[string]int64{"USD": 0, "EUR": 9200, "RUB": 0}, nil).Once()

	result, err := s.service.Exchange(s.ctx, domain.ExchangeOperation{
		UserID:       1,
		FromCurrency: domain.CurrencyUSD,
		ToCurrency:   domain.CurrencyEUR,
		AmountMinor:  10000,
	})

	s.Require().NoError(err)
	s.Require().Equal(int64(9200), result.ExchangedAmountMinor)
	s.Require().Equal(int64(9200), result.NewBalance["EUR"])
}

func (s *UseCaseSuite) TestExchangeUsesCachedRatesAfterGetExchangeRates() {
	s.exchanger.EXPECT().GetRates(s.ctx).Return(map[string]float64{"USD": 1, "EUR": 0.92, "RUB": 90}, nil).Once()
	_, err := s.service.GetExchangeRates(s.ctx)
	s.Require().NoError(err)

	s.storage.EXPECT().Exchange(
		s.ctx,
		int64(1),
		domain.CurrencyUSD,
		domain.CurrencyEUR,
		int64(10000),
		int64(9200),
	).Return(map[string]int64{"USD": 0, "EUR": 9200, "RUB": 0}, nil).Once()

	result, err := s.service.Exchange(s.ctx, domain.ExchangeOperation{
		UserID:       1,
		FromCurrency: domain.CurrencyUSD,
		ToCurrency:   domain.CurrencyEUR,
		AmountMinor:  10000,
	})

	s.Require().NoError(err)
	s.Require().Equal(int64(9200), result.ExchangedAmountMinor)
}

func (s *UseCaseSuite) TestExchangeRefreshesExpiredRatesCache() {
	s.exchanger.EXPECT().GetRates(s.ctx).Return(map[string]float64{"USD": 1, "EUR": 0.92, "RUB": 90}, nil).Once()
	_, err := s.service.GetExchangeRates(s.ctx)
	s.Require().NoError(err)

	s.service.ratesCacheMu.Lock()
	s.service.ratesCachedAt = time.Now().Add(-time.Minute)
	s.service.ratesCacheMu.Unlock()

	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(0.5), nil).Once()
	s.storage.EXPECT().Exchange(
		s.ctx,
		int64(1),
		domain.CurrencyUSD,
		domain.CurrencyEUR,
		int64(10000),
		int64(5000),
	).Return(map[string]int64{"USD": 5000, "EUR": 5000, "RUB": 0}, nil).Once()

	result, err := s.service.Exchange(s.ctx, domain.ExchangeOperation{
		UserID:       1,
		FromCurrency: domain.CurrencyUSD,
		ToCurrency:   domain.CurrencyEUR,
		AmountMinor:  10000,
	})

	s.Require().NoError(err)
	s.Require().Equal(int64(5000), result.ExchangedAmountMinor)
}

func (s *UseCaseSuite) TestExchangePublishesLargeOperation() {
	s.service.publisher = s.publisher
	s.exchanger.EXPECT().GetRate(s.ctx, "RUB", "USD").Return(float64(0.011), nil).Once()
	s.storage.EXPECT().Exchange(
		s.ctx,
		int64(1),
		domain.CurrencyRUB,
		domain.CurrencyUSD,
		int64(3000000),
		int64(33000),
	).Return(map[string]int64{"RUB": 0, "USD": 33000}, nil).Once()
	s.publisher.EXPECT().PublishLargeOperation(s.ctx, mock.MatchedBy(func(event events.LargeOperationEvent) bool {
		return event.OperationType == events.OperationTypeExchange &&
			event.Currency == "RUB" &&
			event.AmountMinor == 3000000 &&
			event.AmountRubMinor == 3000000
	})).Return(nil).Once()

	_, err := s.service.Exchange(s.ctx, domain.ExchangeOperation{
		UserID:       1,
		FromCurrency: domain.CurrencyRUB,
		ToCurrency:   domain.CurrencyUSD,
		AmountMinor:  3000000,
	})

	s.Require().NoError(err)
}

func (s *UseCaseSuite) TestExchangeErrors() {
	_, err := s.service.Exchange(s.ctx, domain.ExchangeOperation{})
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	validOp := domain.ExchangeOperation{UserID: 1, FromCurrency: domain.CurrencyUSD, ToCurrency: domain.CurrencyEUR, AmountMinor: 100}

	s.service.exchanger = nil
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.service.exchanger = s.exchanger
	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(0), errBoom).Once()
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, errBoom)

	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(0), nil).Once()
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(0.001), nil).Once()
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrConvertedAmountTooSmall)

	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(1), nil).Once()
	s.storage.EXPECT().Exchange(
		s.ctx,
		int64(1),
		domain.CurrencyUSD,
		domain.CurrencyEUR,
		int64(100),
		int64(100),
	).Return(nil, domain.ErrInsufficientFunds).Once()
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)

	s.exchanger.EXPECT().GetRate(s.ctx, "USD", "EUR").Return(float64(1), nil).Once()
	s.storage.EXPECT().Exchange(
		s.ctx,
		int64(1),
		domain.CurrencyUSD,
		domain.CurrencyEUR,
		int64(100),
		int64(100),
	).Return(map[string]int64{"GBP": 100}, nil).Once()
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

var errBoom = errors.New("boom")
