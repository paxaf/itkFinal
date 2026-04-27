-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS exchange_rates (
    currency_code VARCHAR(3) PRIMARY KEY,
    units_per_usd NUMERIC(24,4) NOT NULL CHECK (units_per_usd > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (currency_code ~ '^[A-Z]{3}$')
);

INSERT INTO exchange_rates (currency_code, units_per_usd)
VALUES
    ('USD', 1.0000),
    ('RUB', 90.0000),
    ('EUR', 0.9200)
ON CONFLICT (currency_code) DO UPDATE
SET
    units_per_usd = EXCLUDED.units_per_usd,
    updated_at = NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS exchange_rates;
-- +goose StatementEnd
