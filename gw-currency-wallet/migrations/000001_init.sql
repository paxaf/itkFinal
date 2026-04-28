-- +goose Up
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE balances (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency_code VARCHAR(3) NOT NULL CHECK (currency_code IN ('USD', 'RUB', 'EUR')),
    amount_minor BIGINT NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, currency_code)
);

CREATE TABLE wallet_operations (
    id BIGSERIAL PRIMARY KEY,
    operation_id UUID NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    currency_code VARCHAR(3) NOT NULL,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('DEPOSIT', 'WITHDRAW')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPLIED', 'REJECTED')),
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    FOREIGN KEY (user_id, currency_code) REFERENCES balances(user_id, currency_code) ON DELETE CASCADE
);

CREATE INDEX idx_wallet_operations_status_created_id ON wallet_operations(status, created_at, id);
CREATE INDEX idx_wallet_operations_user_currency_status_id ON wallet_operations(user_id, currency_code, status, id);

-- +goose Down
DROP TABLE IF EXISTS wallet_operations;
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS users;
