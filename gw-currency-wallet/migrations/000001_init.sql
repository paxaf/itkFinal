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

-- +goose Down
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS users;
