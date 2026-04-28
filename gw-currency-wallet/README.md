# gw-currency-wallet

Wallet service for registration, authorization, balances, deposits, withdrawals and currency exchange.

Implemented:
- config loading from `config.env` and env variables;
- structured logger;
- PostgreSQL migrations through goose in container entrypoint;
- passwords stored as bcrypt hashes;
- JWT login;
- balances stored as `amount_minor BIGINT` per user and currency;
- synchronous wallet API returning `new_balance`;
- gRPC client for `gw-exchanger`;
- async wallet operation queue and worker batches kept for background processing flows.

HTTP API:
- `POST /api/v1/register`
- `POST /api/v1/login`
- `GET /api/v1/balance`
- `POST /api/v1/wallet/deposit`
- `POST /api/v1/wallet/withdraw`
- `GET /api/v1/exchange/rates`
- `POST /api/v1/exchange`

Protected routes require `Authorization: Bearer <JWT_TOKEN>`.

Tests:
- `make test`
- `make test-integration`
- `make test-skip-integration`
- `go test ./internal/storages/postgres -skip-integration`

Integration tests start PostgreSQL through Docker/testcontainers. If Docker is unavailable, they are skipped automatically.
