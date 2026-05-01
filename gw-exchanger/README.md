# gw-exchanger

`gw-exchanger` - gRPC-сервис курсов валют. Он хранит курсы в PostgreSQL и отдаёт их другим сервисам через контракт из `proto-exchange`.

## Что делает сервис

- Загружает конфигурацию из `config.env` и переменных окружения.
- Подключается к PostgreSQL через `pgxpool`.
- Хранит курсы валют в таблице `exchange_rates`.
- Поддерживает валюты `USD`, `RUB`, `EUR`.
- Отдаёт все курсы или конкретный курс через gRPC.
- Логирует запуск, остановку и ошибки обработки запросов.
- В Docker запускает миграции через `goose` в `entrypoint.sh`.

## Курсы и модель данных

В таблице хранится `units_per_usd`: сколько единиц валюты соответствует одному доллару США.

Пример стартовых значений:

```text
USD = 1.0000
RUB = 90.0000
EUR = 0.9200
```

Курс `from -> to` считается так:

```text
to_units / from_units
```

Например `USD -> RUB = 90 / 1 = 90`, а `RUB -> EUR = 0.92 / 90`.

## gRPC API

Контракт лежит в `proto-exchange/exchange/exchange.proto`.

Методы:

- `GetExchangeRates(Empty) returns (ExchangeRatesResponse)` - вернуть все курсы.
- `GetExchangeRateForCurrency(CurrencyRequest) returns (ExchangeRateResponse)` - вернуть курс между двумя валютами.

Ошибки:

- Некорректная или неподдерживаемая валюта возвращается как `InvalidArgument`.
- Ошибки базы данных возвращаются как `Internal`.

## Конфигурация

Основные переменные:

```env
GRPC_HOST=0.0.0.0
GRPC_PORT=50051
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=exchange
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_SSLMODE=disable
POSTGRES_MAX_OPEN_CONNS=20
POSTGRES_MAX_IDLE_CONNS=10
LOG_LEVEL=debug
```

В Docker Compose `POSTGRES_HOST` должен указывать на имя контейнера базы: `exchanger-db`.

## Локальный запуск

```shell
go run ./cmd -c config.env
```

Для локального запуска нужна доступная PostgreSQL база и применённые миграции.

## Docker

Из корня репозитория:

```shell
docker compose up --build gw-exchanger exchanger-db
```

При старте контейнера `entrypoint.sh` запускает миграции, если `RUN_MIGRATIONS=true`.

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

Они автоматически пропускаются, если Docker недоступен.

## Линтинг и сборка

```shell
make lint
make build
```

Линтер настроен через `.golangci.yml`; тестовые файлы линтером не проверяются.

## Доступы в общем compose

При запуске из корня репозитория сервис доступен с хоста как gRPC endpoint:

```text
localhost:50051
```

Если compose запущен внутри VM, вместо `localhost` нужно использовать IP этой VM:

```text
<vm-ip>:50051
```

PostgreSQL для сервиса доступен:

- внутри docker-сети: `exchanger-db:5432`;
- с хоста: `localhost:5433`;
- база: `exchange`;
- пользователь: `postgres`;
- пароль: `postgres`.

Пример ручной проверки через `grpcurl` из корня репозитория:

```shell
grpcurl -plaintext \
  -import-path ./proto-exchange \
  -proto exchange/exchange.proto \
  localhost:50051 \
  exchange.ExchangeService/GetExchangeRates
```

## Статус перед сдачей

- Сервис запускается в Docker Compose.
- Миграции применяются через `goose` при старте контейнера.
- Unit-тесты и интеграционные PostgreSQL-тесты проходят.
- Моки генерируются через `mockery`.
- Линтер и сборка проходят.
