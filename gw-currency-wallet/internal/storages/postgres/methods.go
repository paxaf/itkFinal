package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	QueryInsertOperation = `INSERT INTO wallet_operations (operation_id, user_id, currency_code, operation_type, amount_minor)
VALUES ($1, $2, $3, $4, $5)`
	QueryListPendingWallets = `SELECT user_id::text || ':' || currency_code AS wallet_key
FROM wallet_operations
WHERE status = 'PENDING'
GROUP BY user_id, currency_code
ORDER BY MIN(id)
LIMIT $1`
	QueryLockWalletBalance = `SELECT amount_minor
FROM balances
WHERE user_id = $1 AND currency_code = $2
FOR UPDATE`
	QuerySelectPendingOperationsForWallet = `SELECT id, operation_type, amount_minor
FROM wallet_operations
WHERE user_id = $1 AND currency_code = $2 AND status = 'PENDING'
ORDER BY id
LIMIT $3
FOR UPDATE SKIP LOCKED`
	QueryUpdateBalanceDelta = `UPDATE balances
SET amount_minor = amount_minor + $1, updated_at = NOW()
WHERE user_id = $2 AND currency_code = $3`
	QueryUpdateOperationResultsBatch = `UPDATE wallet_operations
SET status = $2, error_code = $3, processed_at = clock_timestamp()
WHERE id = ANY($1::bigint[])`
)

const (
	operationStatusApplied  = "APPLIED"
	operationStatusRejected = "REJECTED"

	operationErrorInsufficientFunds = "INSUFFICIENT_FUNDS"
	operationResultGroupsCapacity   = 2
)

type pendingOperation struct {
	id            int64
	operationType domain.OperationType
	amountMinor   int64
}

type operationResult struct {
	id        int64
	status    string
	errorCode *string
}

type operationResultGroup struct {
	status       string
	errorCode    string
	hasErrorCode bool
}

func newOperationResult(id int64, status string, errorCode *string) operationResult {
	return operationResult{
		id:        id,
		status:    status,
		errorCode: errorCode,
	}
}

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
			return domain.User{}, storages.ErrDuplicateUser
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
		return nil, domain.ErrInsufficientFunds
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
		return nil, storages.ErrUserNotFound
	}
	if fromBalance < fromAmountMinor {
		return nil, domain.ErrInsufficientFunds
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

func (p *PgPool) EnqueueOperation(ctx context.Context, operationID string, userID int64, currency domain.Currency, operationType domain.OperationType, amountMinor int64) error {
	_, err := p.pool.Exec(ctx, QueryInsertOperation, operationID, userID, currency, operationType, amountMinor)
	if err != nil {
		if isForeignKeyViolation(err) {
			return storages.ErrUserNotFound
		}
		return fmt.Errorf("insert wallet operation: %w", err)
	}
	return nil
}

func (p *PgPool) ListPendingWallets(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}

	rows, err := p.pool.Query(ctx, QueryListPendingWallets, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending wallets: %w", err)
	}
	defer rows.Close()

	walletKeys := make([]string, 0, limit)
	for rows.Next() {
		var walletKey string
		if err = rows.Scan(&walletKey); err != nil {
			return nil, fmt.Errorf("scan wallet key: %w", err)
		}
		walletKeys = append(walletKeys, walletKey)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending wallets: %w", err)
	}

	return walletKeys, nil
}

func (p *PgPool) ProcessWalletBatch(ctx context.Context, walletKey string, batchSize int) (processed int, err error) {
	if batchSize <= 0 {
		return 0, fmt.Errorf("batch size must be positive")
	}

	userID, currency, err := parseWalletKey(walletKey)
	if err != nil {
		return 0, err
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(tx, &err)

	var balance int64
	err = tx.QueryRow(ctx, QueryLockWalletBalance, userID, currency).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, storages.ErrUserNotFound
		}
		return 0, fmt.Errorf("lock wallet balance: %w", err)
	}

	rows, err := tx.Query(ctx, QuerySelectPendingOperationsForWallet, userID, currency, batchSize)
	if err != nil {
		return 0, fmt.Errorf("select pending operations: %w", err)
	}
	defer rows.Close()

	ops := make([]pendingOperation, 0, batchSize)
	for rows.Next() {
		var op pendingOperation
		var opType string
		if err = rows.Scan(&op.id, &opType, &op.amountMinor); err != nil {
			return 0, fmt.Errorf("scan pending operation: %w", err)
		}
		op.operationType = domain.OperationType(opType)
		ops = append(ops, op)
	}
	if err = rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate pending operations: %w", err)
	}

	if len(ops) == 0 {
		if err = tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit empty batch: %w", err)
		}
		return 0, nil
	}

	runningBalance := balance
	results := make([]operationResult, 0, len(ops))

	for _, op := range ops {
		switch op.operationType {
		case domain.OperationDeposit:
			runningBalance += op.amountMinor
			results = append(results, newOperationResult(op.id, operationStatusApplied, nil))
		case domain.OperationWithdraw:
			if runningBalance >= op.amountMinor {
				runningBalance -= op.amountMinor
				results = append(results, newOperationResult(op.id, operationStatusApplied, nil))
			} else {
				errCode := operationErrorInsufficientFunds
				results = append(results, newOperationResult(op.id, operationStatusRejected, &errCode))
			}
		default:
			return 0, fmt.Errorf("unexpected operation type: %s", op.operationType)
		}
	}

	delta := runningBalance - balance
	if delta != 0 {
		var res pgconn.CommandTag
		res, err = tx.Exec(ctx, QueryUpdateBalanceDelta, delta, userID, currency)
		if err != nil {
			return 0, fmt.Errorf("update balance delta: %w", err)
		}
		if res.RowsAffected() == 0 {
			return 0, storages.ErrUserNotFound
		}
	}

	if err = applyOperationResults(ctx, tx, results); err != nil {
		return 0, err
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit batch: %w", err)
	}

	return len(ops), nil
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

func parseWalletKey(walletKey string) (int64, domain.Currency, error) {
	parts := strings.Split(walletKey, ":")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid wallet key: %s", walletKey)
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID <= 0 {
		return 0, "", fmt.Errorf("invalid wallet user id: %s", parts[0])
	}

	currency, err := domain.NormalizeCurrency(parts[1])
	if err != nil {
		return 0, "", err
	}

	return userID, currency, nil
}

func applyOperationResults(ctx context.Context, tx pgx.Tx, results []operationResult) error {
	groupedIDs := make(map[operationResultGroup][]int64, operationResultGroupsCapacity)

	for _, result := range results {
		group := operationResultGroup{status: result.status}
		if result.errorCode != nil {
			group.errorCode = *result.errorCode
			group.hasErrorCode = true
		}
		groupedIDs[group] = append(groupedIDs[group], result.id)
	}

	for group, ids := range groupedIDs {
		if err := updateOperationResultsBatch(ctx, tx, ids, group); err != nil {
			return err
		}
	}

	return nil
}

func updateOperationResultsBatch(ctx context.Context, tx pgx.Tx, ids []int64, group operationResultGroup) error {
	if len(ids) == 0 {
		return nil
	}

	var errorCode *string
	if group.hasErrorCode {
		errCode := group.errorCode
		errorCode = &errCode
	}

	res, err := tx.Exec(ctx, QueryUpdateOperationResultsBatch, ids, group.status, errorCode)
	if err != nil {
		return fmt.Errorf("update operation results batch: %w", err)
	}
	if int(res.RowsAffected()) != len(ids) {
		return fmt.Errorf("update operation results batch: rows affected mismatch: got %d want %d", res.RowsAffected(), len(ids))
	}

	return nil
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

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
