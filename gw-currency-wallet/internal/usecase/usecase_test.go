package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
)

type UseCaseSuite struct {
	suite.Suite

	ctx       context.Context
	storage   *fakeStorage
	tokens    *fakeTokenManager
	exchanger *fakeExchanger
	service   *Service
}

func TestUseCaseSuite(t *testing.T) {
	suite.Run(t, new(UseCaseSuite))
}

func (s *UseCaseSuite) SetupTest() {
	s.ctx = context.Background()
	s.storage = &fakeStorage{}
	s.tokens = &fakeTokenManager{token: "token"}
	s.exchanger = &fakeExchanger{}
	s.service = New(s.storage, s.tokens, s.exchanger)
}

func (s *UseCaseSuite) TestRegisterHashesPasswordAndCreatesUser() {
	s.storage.createUserFn = func(ctx context.Context, username string, email string, passwordHash string) (domain.User, error) {
		s.Require().Equal("paxaf", username)
		s.Require().Equal("paxaf@example.com", email)
		s.Require().NoError(bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("secret1")))
		return domain.User{ID: 1, Username: username, Email: email}, nil
	}

	user, err := s.service.Register(s.ctx, domain.RegisterUser{
		Username: " paxaf ",
		Email:    " paxaf@example.com ",
		Password: "secret1",
	})

	s.Require().NoError(err)
	s.Require().Equal(int64(1), user.ID)
}

func (s *UseCaseSuite) TestRegisterReturnsDuplicateUser() {
	s.storage.createUserFn = func(ctx context.Context, username string, email string, passwordHash string) (domain.User, error) {
		return domain.User{}, storages.ErrDuplicateUser
	}

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
	s.storage.credentials = domain.UserCredentials{
		User:         domain.User{ID: 42, Username: "paxaf", Email: "paxaf@example.com"},
		PasswordHash: string(hash),
	}

	token, err := s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().NoError(err)
	s.Require().Equal("token", token)
	s.Require().Equal(int64(42), s.tokens.userID)
}

func (s *UseCaseSuite) TestLoginReturnsStorageError() {
	s.storage.credentialsErr = errBoom

	_, err := s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().ErrorIs(err, errBoom)
}

func (s *UseCaseSuite) TestLoginReturnsTokenError() {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret1"), bcrypt.DefaultCost)
	s.Require().NoError(err)
	s.storage.credentials = domain.UserCredentials{User: domain.User{ID: 42}, PasswordHash: string(hash)}
	s.tokens.err = errBoom

	_, err = s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().ErrorIs(err, errBoom)
}

func (s *UseCaseSuite) TestLoginReturnsInvalidCredentialsOnWrongPassword() {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret1"), bcrypt.DefaultCost)
	s.Require().NoError(err)
	s.storage.credentials = domain.UserCredentials{User: domain.User{ID: 42}, PasswordHash: string(hash)}

	_, err = s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "badpass"})

	s.Require().ErrorIs(err, domain.ErrInvalidCredentials)
}

func (s *UseCaseSuite) TestLoginReturnsInvalidCredentialsWhenUserMissing() {
	_, err := s.service.Login(s.ctx, domain.LoginUser{Username: "paxaf", Password: "secret1"})

	s.Require().ErrorIs(err, domain.ErrInvalidCredentials)
}

func (s *UseCaseSuite) TestGetBalanceNormalizesMissingCurrencies() {
	s.storage.balances = map[string]int64{"USD": 10000}

	balance, err := s.service.GetBalance(s.ctx, 1)

	s.Require().NoError(err)
	s.Require().Equal(map[string]int64{"USD": 10000, "RUB": 0, "EUR": 0}, balance)
}

func (s *UseCaseSuite) TestGetBalanceErrors() {
	_, err := s.service.GetBalance(s.ctx, 0)
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	s.storage.balancesErr = errBoom
	_, err = s.service.GetBalance(s.ctx, 1)
	s.Require().ErrorIs(err, errBoom)

	s.storage.balancesErr = nil
	s.storage.balances = map[string]int64{"GBP": 100}
	_, err = s.service.GetBalance(s.ctx, 1)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

func (s *UseCaseSuite) TestDepositCallsStorage() {
	s.storage.depositFn = func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
		s.Require().Equal(int64(1), userID)
		s.Require().Equal(domain.CurrencyUSD, currency)
		s.Require().Equal(int64(10050), amountMinor)
		return map[string]int64{"USD": 10050, "RUB": 0, "EUR": 0}, nil
	}

	balance, err := s.service.Deposit(s.ctx, 1, "usd", 10050)

	s.Require().NoError(err)
	s.Require().Equal(int64(10050), balance["USD"])
}

func (s *UseCaseSuite) TestDepositErrors() {
	_, err := s.service.Deposit(s.ctx, 1, "GBP", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)

	_, err = s.service.Deposit(s.ctx, 0, "USD", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	s.storage.depositFn = func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
		return nil, errBoom
	}
	_, err = s.service.Deposit(s.ctx, 1, "USD", 100)
	s.Require().ErrorIs(err, errBoom)

	s.storage.depositFn = func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
		return map[string]int64{"GBP": 100}, nil
	}
	_, err = s.service.Deposit(s.ctx, 1, "USD", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

func (s *UseCaseSuite) TestWithdrawReturnsInsufficientFunds() {
	s.storage.withdrawFn = func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
		return nil, domain.ErrInsufficientFunds
	}

	_, err := s.service.Withdraw(s.ctx, 1, "USD", 10050)

	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)
}

func (s *UseCaseSuite) TestWithdrawSuccessAndErrors() {
	s.storage.withdrawFn = func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
		return map[string]int64{"USD": 5000}, nil
	}
	balance, err := s.service.Withdraw(s.ctx, 1, "USD", 100)
	s.Require().NoError(err)
	s.Require().Equal(int64(5000), balance["USD"])

	_, err = s.service.Withdraw(s.ctx, 1, "GBP", 100)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)

	_, err = s.service.Withdraw(s.ctx, 1, "USD", 0)
	s.Require().ErrorIs(err, domain.ErrInvalidAmount)
}

func (s *UseCaseSuite) TestWalletOperation() {
	err := s.service.WalletOperation(s.ctx, domain.WalletOperation{
		UserID:        1,
		Currency:      domain.CurrencyUSD,
		OperationType: domain.OperationDeposit,
		AmountMinor:   100,
	})
	s.Require().NoError(err)
	s.Require().Equal(int64(1), s.storage.enqueuedUserID)
	s.Require().NotEmpty(s.storage.enqueuedOperationID)

	err = s.service.WalletOperation(s.ctx, domain.WalletOperation{})
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	s.storage.enqueueErr = errBoom
	err = s.service.WalletOperation(s.ctx, domain.WalletOperation{
		UserID:        1,
		Currency:      domain.CurrencyUSD,
		OperationType: domain.OperationDeposit,
		AmountMinor:   100,
	})
	s.Require().ErrorIs(err, errBoom)
}

func (s *UseCaseSuite) TestGetExchangeRates() {
	s.exchanger.rates = map[string]float64{"USD": 1, "EUR": 0.92, "RUB": 90}

	rates, err := s.service.GetExchangeRates(s.ctx)

	s.Require().NoError(err)
	s.Require().Equal(float64(0.92), rates["EUR"])
}

func (s *UseCaseSuite) TestGetExchangeRatesErrors() {
	s.service.exchanger = nil
	_, err := s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.service.exchanger = s.exchanger
	s.exchanger.err = errBoom
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, errBoom)

	s.exchanger.err = nil
	s.exchanger.rates = map[string]float64{"GBP": 1}
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)

	s.exchanger.rates = map[string]float64{"USD": 0}
	_, err = s.service.GetExchangeRates(s.ctx)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)
}

func (s *UseCaseSuite) TestExchangeConvertsAndStoresAtomically() {
	s.exchanger.rate = 0.92
	s.storage.exchangeFn = func(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error) {
		s.Require().Equal(int64(1), userID)
		s.Require().Equal(domain.CurrencyUSD, fromCurrency)
		s.Require().Equal(domain.CurrencyEUR, toCurrency)
		s.Require().Equal(int64(10000), fromAmountMinor)
		s.Require().Equal(int64(9200), toAmountMinor)
		return map[string]int64{"USD": 0, "EUR": 9200, "RUB": 0}, nil
	}

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

func (s *UseCaseSuite) TestExchangeErrors() {
	_, err := s.service.Exchange(s.ctx, domain.ExchangeOperation{})
	s.Require().ErrorIs(err, domain.ErrInvalidUserID)

	validOp := domain.ExchangeOperation{UserID: 1, FromCurrency: domain.CurrencyUSD, ToCurrency: domain.CurrencyEUR, AmountMinor: 100}

	s.service.exchanger = nil
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.service.exchanger = s.exchanger
	s.exchanger.err = errBoom
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, errBoom)

	s.exchanger.err = nil
	s.exchanger.rate = 0
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrExchangeRateUnavailable)

	s.exchanger.rate = 0.001
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrConvertedAmountTooSmall)

	s.exchanger.rate = 1
	s.storage.exchangeFn = func(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error) {
		return nil, domain.ErrInsufficientFunds
	}
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrInsufficientFunds)

	s.storage.exchangeFn = func(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error) {
		return map[string]int64{"GBP": 100}, nil
	}
	_, err = s.service.Exchange(s.ctx, validOp)
	s.Require().ErrorIs(err, domain.ErrInvalidCurrency)
}

func (s *UseCaseSuite) TestProcessorService() {
	processor := NewProcessor(s.storage, ProcessorConfig{})
	s.storage.pendingWallets = []string{"1:USD"}

	wallets, err := processor.ListPendingWallets(s.ctx, 10)
	s.Require().NoError(err)
	s.Require().Equal([]string{"1:USD"}, wallets)

	err = processor.ProcessWallet(s.ctx, "1:USD")
	s.Require().NoError(err)
	s.Require().Equal("1:USD", s.storage.processedWalletKey)
	s.Require().Equal(128, s.storage.processedBatchSize)
}

func (s *UseCaseSuite) TestProcessorServiceErrors() {
	processor := NewProcessor(s.storage, ProcessorConfig{BatchSize: 7})
	canceledCtx, cancel := context.WithCancel(s.ctx)
	cancel()

	_, err := processor.ListPendingWallets(canceledCtx, 10)
	s.Require().ErrorIs(err, context.Canceled)

	err = processor.ProcessWallet(canceledCtx, "1:USD")
	s.Require().ErrorIs(err, context.Canceled)

	s.storage.processErr = errBoom
	err = processor.ProcessWallet(s.ctx, "1:USD")
	s.Require().ErrorIs(err, errBoom)
	s.Require().Contains(err.Error(), "process wallet")
}

type fakeTokenManager struct {
	token  string
	userID int64
	err    error
}

func (f *fakeTokenManager) Generate(userID int64) (string, error) {
	f.userID = userID
	return f.token, f.err
}

type fakeExchanger struct {
	rates map[string]float64
	rate  float64
	err   error
}

func (f *fakeExchanger) GetRates(ctx context.Context) (map[string]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rates, nil
}

func (f *fakeExchanger) GetRate(ctx context.Context, fromCurrency string, toCurrency string) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.rate, nil
}

type fakeStorage struct {
	credentials         domain.UserCredentials
	credentialsErr      error
	balances            map[string]int64
	balancesErr         error
	enqueueErr          error
	enqueuedOperationID string
	enqueuedUserID      int64
	pendingWallets      []string
	listErr             error
	processErr          error
	processedWalletKey  string
	processedBatchSize  int

	createUserFn func(ctx context.Context, username string, email string, passwordHash string) (domain.User, error)
	depositFn    func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error)
	withdrawFn   func(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error)
	exchangeFn   func(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error)
}

func (f *fakeStorage) CreateUser(ctx context.Context, username string, email string, passwordHash string) (domain.User, error) {
	if f.createUserFn != nil {
		return f.createUserFn(ctx, username, email, passwordHash)
	}
	return domain.User{ID: 1, Username: username, Email: email}, nil
}

func (f *fakeStorage) GetUserCredentialsByUsername(ctx context.Context, username string) (domain.UserCredentials, error) {
	if f.credentialsErr != nil {
		return domain.UserCredentials{}, f.credentialsErr
	}
	if f.credentials.ID == 0 {
		return domain.UserCredentials{}, storages.ErrUserNotFound
	}
	return f.credentials, nil
}

func (f *fakeStorage) GetBalances(ctx context.Context, userID int64) (map[string]int64, error) {
	if f.balancesErr != nil {
		return nil, f.balancesErr
	}
	if f.balances == nil {
		return nil, storages.ErrUserNotFound
	}
	return f.balances, nil
}

func (f *fakeStorage) Deposit(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
	if f.depositFn != nil {
		return f.depositFn(ctx, userID, currency, amountMinor)
	}
	return f.balances, nil
}

func (f *fakeStorage) Withdraw(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
	if f.withdrawFn != nil {
		return f.withdrawFn(ctx, userID, currency, amountMinor)
	}
	return f.balances, nil
}

func (f *fakeStorage) Exchange(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error) {
	if f.exchangeFn != nil {
		return f.exchangeFn(ctx, userID, fromCurrency, toCurrency, fromAmountMinor, toAmountMinor)
	}
	return f.balances, nil
}

func (f *fakeStorage) EnqueueOperation(ctx context.Context, operationID string, userID int64, currency domain.Currency, operationType domain.OperationType, amountMinor int64) error {
	f.enqueuedOperationID = operationID
	f.enqueuedUserID = userID
	return f.enqueueErr
}

func (f *fakeStorage) ListPendingWallets(ctx context.Context, limit int) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.pendingWallets, nil
}

func (f *fakeStorage) ProcessWalletBatch(ctx context.Context, walletKey string, batchSize int) (int, error) {
	f.processedWalletKey = walletKey
	f.processedBatchSize = batchSize
	if f.processErr != nil {
		return 0, f.processErr
	}
	return 1, nil
}

func (f *fakeStorage) Close() error {
	return nil
}

var errBoom = errors.New("boom")
