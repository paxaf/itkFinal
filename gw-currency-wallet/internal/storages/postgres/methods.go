package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
)

const rollbackTimeout = time.Second

const (
	QueryInsertUser = `INSERT INTO users (username, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, username, email`
	QueryInsertBalance = `INSERT INTO balances (user_id, currency_code, amount_minor)
VALUES ($1, $2, 0)
ON CONFLICT (user_id, currency_code) DO NOTHING`
	QueryGetUserCredentialsByUsername = `SELECT id, username, email, password_hash
FROM users
WHERE username = $1`
	QueryGetBalances = `SELECT currency_code, amount_minor
FROM balances
WHERE user_id = $1
ORDER BY currency_code`
	QueryLockBalance = `SELECT amount_minor
FROM balances
WHERE user_id = $1 AND currency_code = $2
FOR UPDATE`
	QueryLockBalancesForExchange = `SELECT currency_code, amount_minor
FROM balances
WHERE user_id = $1 AND currency_code IN ($2, $3)
ORDER BY currency_code
FOR UPDATE`
	QueryUpdateBalanceDelta = `UPDATE balances
SET amount_minor = amount_minor + $1, updated_at = NOW()
WHERE user_id = $2 AND currency_code = $3`
)

func (p *PgPool) CreateUser(ctx context.Context, username string, email string, passwordHash string) (domain.User, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(tx, &err)

	var user domain.User
	err = tx.QueryRow(ctx, QueryInsertUser, username, email, passwordHash).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		if isUniqueViolation(err) {
			err = storages.ErrDuplicateUser
			return domain.User{}, err
		}
		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}

	for _, currency := range domain.SupportedCurrencies {
		if _, err = tx.Exec(ctx, QueryInsertBalance, user.ID, currency); err != nil {
			return domain.User{}, fmt.Errorf("insert balance %s: %w", currency, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit create user: %w", err)
	}

	return user, nil
}

func (p *PgPool) GetUserCredentialsByUsername(ctx context.Context, username string) (domain.UserCredentials, error) {
	var user domain.UserCredentials
	err := p.pool.QueryRow(ctx, QueryGetUserCredentialsByUsername, username).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UserCredentials{}, storages.ErrUserNotFound
		}
		return domain.UserCredentials{}, fmt.Errorf("get user credentials: %w", err)
	}

	return user, nil
}

func (p *PgPool) GetBalances(ctx context.Context, userID int64) (map[string]int64, error) {
	rows, err := p.pool.Query(ctx, QueryGetBalances, userID)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[string]int64, len(domain.SupportedCurrencies))
	for rows.Next() {
		var currency string
		var amountMinor int64
		if err = rows.Scan(&currency, &amountMinor); err != nil {
			return nil, fmt.Errorf("scan balance: %w", err)
		}
		balances[currency] = amountMinor
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate balances: %w", err)
	}
	if len(balances) == 0 {
		return nil, storages.ErrUserNotFound
	}

	return balances, nil
}

func (p *PgPool) Deposit(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
	if amountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(tx, &err)

	if _, err = lockBalance(ctx, tx, userID, currency); err != nil {
		return nil, err
	}

	if err = updateBalanceDelta(ctx, tx, userID, currency, amountMinor); err != nil {
		return nil, err
	}

	balances, err := getBalancesTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit deposit: %w", err)
	}

	return balances, nil
}

func (p *PgPool) Withdraw(ctx context.Context, userID int64, currency domain.Currency, amountMinor int64) (map[string]int64, error) {
	if amountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(tx, &err)

	balance, err := lockBalance(ctx, tx, userID, currency)
	if err != nil {
		return nil, err
	}
	if balance < amountMinor {
		err = domain.ErrInsufficientFunds
		return nil, err
	}

	if err = updateBalanceDelta(ctx, tx, userID, currency, -amountMinor); err != nil {
		return nil, err
	}

	balances, err := getBalancesTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit withdraw: %w", err)
	}

	return balances, nil
}

func (p *PgPool) Exchange(ctx context.Context, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency, fromAmountMinor int64, toAmountMinor int64) (map[string]int64, error) {
	if fromAmountMinor <= 0 || toAmountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if fromCurrency == toCurrency {
		return nil, domain.ErrSameCurrency
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(tx, &err)

	lockedBalances, err := lockExchangeBalances(ctx, tx, userID, fromCurrency, toCurrency)
	if err != nil {
		return nil, err
	}
	fromBalance, ok := lockedBalances[fromCurrency]
	if !ok {
		err = storages.ErrUserNotFound
		return nil, err
	}
	if fromBalance < fromAmountMinor {
		err = domain.ErrInsufficientFunds
		return nil, err
	}

	if err = updateBalanceDelta(ctx, tx, userID, fromCurrency, -fromAmountMinor); err != nil {
		return nil, err
	}
	if err = updateBalanceDelta(ctx, tx, userID, toCurrency, toAmountMinor); err != nil {
		return nil, err
	}

	balances, err := getBalancesTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit exchange: %w", err)
	}

	return balances, nil
}

func lockBalance(ctx context.Context, tx pgx.Tx, userID int64, currency domain.Currency) (int64, error) {
	var balance int64
	err := tx.QueryRow(ctx, QueryLockBalance, userID, currency).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, storages.ErrUserNotFound
		}
		return 0, fmt.Errorf("lock balance: %w", err)
	}

	return balance, nil
}

func lockExchangeBalances(ctx context.Context, tx pgx.Tx, userID int64, fromCurrency domain.Currency, toCurrency domain.Currency) (map[domain.Currency]int64, error) {
	rows, err := tx.Query(ctx, QueryLockBalancesForExchange, userID, fromCurrency, toCurrency)
	if err != nil {
		return nil, fmt.Errorf("lock exchange balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[domain.Currency]int64, 2)
	for rows.Next() {
		var currency string
		var amountMinor int64
		if err = rows.Scan(&currency, &amountMinor); err != nil {
			return nil, fmt.Errorf("scan locked exchange balance: %w", err)
		}
		code, normalizeErr := domain.NormalizeCurrency(currency)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		balances[code] = amountMinor
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked exchange balances: %w", err)
	}
	if len(balances) != 2 {
		return nil, storages.ErrUserNotFound
	}

	return balances, nil
}

func updateBalanceDelta(ctx context.Context, tx pgx.Tx, userID int64, currency domain.Currency, delta int64) error {
	res, err := tx.Exec(ctx, QueryUpdateBalanceDelta, delta, userID, currency)
	if err != nil {
		return fmt.Errorf("update balance delta: %w", err)
	}
	if res.RowsAffected() == 0 {
		return storages.ErrUserNotFound
	}

	return nil
}

func getBalancesTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string]int64, error) {
	rows, err := tx.Query(ctx, QueryGetBalances, userID)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[string]int64, len(domain.SupportedCurrencies))
	for rows.Next() {
		var currency string
		var amountMinor int64
		if err = rows.Scan(&currency, &amountMinor); err != nil {
			return nil, fmt.Errorf("scan balance: %w", err)
		}
		balances[currency] = amountMinor
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate balances: %w", err)
	}
	if len(balances) == 0 {
		return nil, storages.ErrUserNotFound
	}

	return balances, nil
}

func rollback(tx pgx.Tx, err *error) {
	rb := func() {
		rbCtx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
		defer cancel()
		_ = tx.Rollback(rbCtx)
	}

	if p := recover(); p != nil {
		rb()
		panic(p)
	}

	if err != nil && *err != nil {
		rb()
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
