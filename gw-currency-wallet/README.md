# gw-currency-wallet

`gw-currency-wallet` - REST API сервиса кошелька и обмена валют. Он отвечает за пользователей, JWT-авторизацию, балансы, пополнение, вывод средств, обмен валют и отправку событий о крупных операциях в Kafka.

## Что делает сервис

- Регистрирует пользователей и хранит пароль только в виде bcrypt-хэша.
- Авторизует пользователя и выдаёт JWT.
- Защищает приватные ручки через `Authorization: Bearer <token>`.
- Хранит балансы пользователей в PostgreSQL в minor units: копейки и центы.
- Выполняет пополнение, вывод и обмен валют в транзакциях.
- Получает курсы валют из `gw-exchanger` по gRPC.
- Кэширует полученные курсы на короткое время после запроса `/api/v1/exchange/rates`.
- Публикует в Kafka событие о крупной операции, если сумма в RUB minor больше или равна `LARGE_OPERATION_THRESHOLD_RUB_MINOR`.
- Логирует HTTP-операции, ошибки бизнес-логики и ошибки публикации событий.

## API

Публичные ручки:

- `GET /api/v1/health`
- `POST /api/v1/register`
- `POST /api/v1/login`
- `GET /swagger/index.html`

Защищённые JWT ручки:

- `GET /api/v1/balance`
- `POST /api/v1/wallet/deposit`
- `POST /api/v1/wallet/withdraw`
- `GET /api/v1/exchange/rates`
- `POST /api/v1/exchange`

Для защищённых ручек нужен заголовок:

```http
Authorization: Bearer <JWT_TOKEN>
```

## Денежная модель

Суммы внутри сервиса хранятся как `int64` в minor units:

```text
100.50 USD -> 10050
50.00 RUB  -> 5000
```

Так мы не используем `float` для денег и избегаем ошибок округления при хранении балансов.

## Обмен валют

Для обмена сервис получает курс из `gw-exchanger`.

Если перед обменом пользователь запрашивал `/api/v1/exchange/rates`, то курсы некоторое время берутся из локального кэша. Если кэша нет или он устарел, выполняется gRPC-запрос за конкретным курсом.

## События крупных операций

После успешных операций `deposit`, `withdraw`, `exchange` сервис вызывает внутреннюю проверку крупной операции.

Текущая логика:

```text
успешная денежная операция
        |
        v
посчитать сумму в RUB minor
        |
        v
если сумма >= 30 000 RUB -> отправить событие в Kafka topic wallet.large-operations
если сумма меньше -> ничего не отправлять
```

Событие отправляется в JSON-формате:

```json
{
  "event_id": "uuid",
  "user_id": 1,
  "operation_type": "DEPOSIT",
  "currency": "RUB",
  "amount_minor": 5000000,
  "amount_rub_minor": 5000000,
  "created_at": "2026-04-30T12:00:00Z"
}
```

Если Kafka временно недоступна при публикации, клиентская операция не откатывается. Ошибка публикации логируется, потому что сама денежная операция уже успешно применена в PostgreSQL.

## Конфигурация

Основные переменные:

```env
HTTP_HOST=0.0.0.0
HTTP_PORT=8080
HTTP_ACCESS_LOG=true
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=wallet
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_SSLMODE=disable
JWT_SECRET=local-dev-secret
JWT_TOKEN_TTL_MINUTES=60
EXCHANGER_GRPC_HOST=localhost
EXCHANGER_GRPC_PORT=50051
KAFKA_BROKERS=localhost:9092
KAFKA_OPERATIONS_TOPIC=wallet.operations
KAFKA_LARGE_OPERATIONS_TOPIC=wallet.large-operations
LARGE_OPERATION_THRESHOLD_RUB_MINOR=3000000
LOG_LEVEL=debug
```

В Docker Compose сервис обращается к зависимостям по именам контейнеров:

- PostgreSQL: `wallet-db`
- Exchanger gRPC: `gw-exchanger:50051`
- Kafka: `kafka:9092`

## Swagger

Swagger UI доступен по адресу:

```text
/swagger/index.html
```

Перегенерация документации:

```shell
make swagger
```

## Локальный запуск

```shell
go run ./cmd -c config.env
```

Для полноценного запуска нужны PostgreSQL, `gw-exchanger` и Kafka.

## Docker

Из корня репозитория:

```shell
docker compose up --build gw-currency-wallet
```

`entrypoint.sh` запускает миграции PostgreSQL через `goose`, если `RUN_MIGRATIONS=true`.

## Тесты и моки

Моки генерируются через `mockery` по конфигу `.mockery.yaml`.

```shell
make mocks
make test
```

`make test` сначала перегенерирует моки, затем запускает unit-тесты без интеграционных.

Интеграционные тесты PostgreSQL:

```shell
make test-integration
```

Если Docker недоступен, интеграционные тесты автоматически пропускаются.

## Линтинг и сборка

```shell
make lint
make build
```

Линтер настроен через `.golangci.yml`; тестовые файлы линтером не проверяются.

## Доступы в общем compose

Основной адрес REST API при локальном запуске compose:

```text
http://localhost:8080
```

Если compose запущен внутри VM, вместо `localhost` нужно использовать IP этой VM:

```text
http://<vm-ip>:8080
```

Swagger доступен по адресу:

```text
http://localhost:8080/swagger/index.html
```

Зависимости внутри docker-сети:

- PostgreSQL: `wallet-db:5432`;
- Exchanger gRPC: `gw-exchanger:50051`;
- Kafka: `kafka:9092`.

Доступ к PostgreSQL wallet с хоста:

- адрес: `localhost:5434`;
- база: `wallet`;
- пользователь: `postgres`;
- пароль: `postgres`.

## Быстрая проверка через Postman

1. `GET http://localhost:8080/api/v1/health`.
2. `POST http://localhost:8080/api/v1/register` с `username`, `email`, `password`.
3. `POST http://localhost:8080/api/v1/login`, сохранить `token`.
4. Для приватных ручек добавить заголовок `Authorization: Bearer <token>`.
5. Проверить `GET /api/v1/balance`, `POST /api/v1/wallet/deposit`, `POST /api/v1/wallet/withdraw`, `GET /api/v1/exchange/rates`, `POST /api/v1/exchange`.
6. Для проверки notification и analytics выполнить операцию на сумму от `30000 RUB`.

## События для analytics

Помимо крупных операций сервис публикует события всех денежных операций в Kafka topic `wallet.operations`. Эти события читает `gw-analytics` и сохраняет в Elasticsearch. В событии есть тип операции, статус, сумма, валюта, сумма в RUB minor, время создания и ошибка для неуспешной операции.

## Статус перед сдачей

- Приватные ручки защищены JWT.
- Пароли хранятся только как bcrypt-хэш.
- Денежные значения хранятся в minor units.
- PostgreSQL-операции выполняются транзакционно.
- gRPC-клиент exchanger подключен.
- Kafka publisher отправляет события для notification и analytics.
- Swagger-документация доступна.
- Unit-тесты и интеграционные PostgreSQL-тесты проходят.
- Моки генерируются через `mockery`.
- Линтер и сборка проходят.
