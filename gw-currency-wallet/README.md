# gw-currency-wallet

`gw-currency-wallet` - HTTP-сервис кошелька и обмена валют.

Сервис отвечает за регистрацию и авторизацию пользователей, получение баланса, пополнение, вывод средств, получение курсов валют и обмен валют. Для хранения данных используется PostgreSQL, для авторизации - JWT, для получения курсов - gRPC-вызовы в `gw-exchanger`.

## Возможности

- загрузка конфигурации из `config.env` и переменных окружения;
- структурированное логирование;
- миграции PostgreSQL через `goose` в `entrypoint.sh`;
- хранение паролей в виде bcrypt-хэшей;
- выдача JWT-токена при логине;
- хранение балансов в minor units: копейки/центы в `amount_minor`;
- синхронные операции кошелька с возвратом `new_balance`;
- gRPC-клиент для обращения к `gw-exchanger`;
- кэш курсов после запроса `/api/v1/exchange/rates`;
- фоновые worker/batch-механизмы для очереди операций кошелька.

## HTTP API

- `POST /api/v1/register` - регистрация пользователя.
- `POST /api/v1/login` - авторизация и получение JWT.
- `GET /api/v1/balance` - получение баланса пользователя.
- `POST /api/v1/wallet/deposit` - пополнение кошелька.
- `POST /api/v1/wallet/withdraw` - вывод средств.
- `GET /api/v1/exchange/rates` - получение курсов валют.
- `POST /api/v1/exchange` - обмен валют.
- `GET /api/v1/health` - проверка состояния сервиса.

Защищенные ручки требуют заголовок:

```http
Authorization: Bearer <JWT_TOKEN>
```

Публичные ручки: `health`, `register`, `login`, `swagger`.

## Документация API

- Swagger UI: `GET /swagger/index.html`
- JSON-спецификация: `GET /swagger/doc.json`
- YAML-спецификация в репозитории: `docs/swagger.yaml`
- Обновить Swagger-документацию: `make swagger`

## Локальный запуск

```shell
go run ./cmd -c config.env
```

## Docker

Сервис рассчитан на запуск из корневого `docker-compose.yml` вместе с `wallet-db` и `gw-exchanger`.

```shell
docker compose up --build gw-currency-wallet
```

При старте контейнера `entrypoint.sh` запускает миграции, если `RUN_MIGRATIONS=true`.

## Тесты

```shell
make test
```

Интеграционные тесты PostgreSQL:

```shell
make test-integration
```

Интеграционные тесты автоматически пропускаются, если Docker недоступен.

## Сборка

```shell
go build -o main ./cmd
```
