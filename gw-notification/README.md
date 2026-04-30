# gw-notification

`gw-notification` - сервис обработки событий о крупных денежных операциях.

Сервис читает события из Kafka, валидирует их, сохраняет историю крупных операций в MongoDB и подтверждает Kafka offset только после успешной обработки. Источником событий будет `gw-currency-wallet`: он отправляет в Kafka только операции, которые превышают заданный порог, например 30 000 RUB.

## Назначение

- получать события крупных операций из Kafka topic;
- обрабатывать сообщения микробатчами;
- сохранять валидные события в MongoDB;
- не создавать дубликаты при повторной доставке сообщений;
- логировать обработку batch и ошибки;
- корректно завершаться по `SIGINT` и `SIGTERM`.

## Поток Данных

```text
gw-currency-wallet -> Kafka topic -> gw-notification -> MongoDB
```

`gw-notification` не принимает HTTP или gRPC запросы. Это фоновый Kafka consumer.

## Контракт События

Событие приходит в Kafka как JSON:

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

Поля:

- `event_id` - уникальный идентификатор события.
- `user_id` - идентификатор пользователя.
- `operation_type` - тип операции, например `DEPOSIT`, `WITHDRAW`, `EXCHANGE`.
- `currency` - валюта исходной операции.
- `amount_minor` - сумма операции в minor units.
- `amount_rub_minor` - сумма операции в рублях в minor units.
- `created_at` - время создания события.

## Идемпотентность

MongoDB collection получает уникальный индекс по полю `event_id`.

Если Kafka повторно доставит уже сохранённое событие, MongoDB вернет duplicate key error, а storage-слой обработает это как успешный результат. Это защищает сервис от дублей при повторной доставке сообщений или ошибках Kafka commit.

## Batch-Обработка

Consumer собирает batch так:

- ждёт первое сообщение;
- после первого сообщения добирает batch до `KAFKA_BATCH_SIZE`;
- если batch не успел заполниться, завершает добор по `KAFKA_BATCH_WAIT_MS`;
- валидные события отправляет в usecase;
- битый JSON логирует и пропускает;
- после успешного сохранения подтверждает Kafka offset через commit.

Если сохранение в MongoDB завершилось ошибкой, batch не подтверждается.

## Конфигурация

Конфиг загружается из `config.env` и переменных окружения.

Основные переменные:

```env
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=wallet.large-operations
KAFKA_GROUP_ID=gw-notification
KAFKA_MIN_BYTES=1
KAFKA_MAX_BYTES=10485760
KAFKA_MAX_WAIT_MS=500
KAFKA_BATCH_SIZE=128
KAFKA_BATCH_WAIT_MS=50

MONGO_URI=
MONGO_HOST=localhost
MONGO_PORT=27017
MONGO_USER=mongo
MONGO_PASSWORD=mongo
MONGO_AUTH_SOURCE=admin
MONGO_DB=notification
MONGO_COLLECTION=large_operations
MONGO_CONNECT_TIMEOUT_MS=5000

LOG_LEVEL=debug
```

В Docker Compose сервисы должны обращаться к Kafka и MongoDB по именам сервисов, например:

```env
KAFKA_BROKERS=kafka:9092
MONGO_HOST=notification-mongo
```

## Структура

- `cmd` - точка входа.
- `internal/app` - сборка приложения и graceful shutdown.
- `internal/config` - загрузка и валидация конфигурации.
- `internal/logger` - общий logger на zerolog.
- `internal/domain` - доменная модель события.
- `internal/usecase` - обработка и валидация событий.
- `internal/storages` - интерфейсы хранилища.
- `internal/storages/mongo` - MongoDB storage.
- `internal/transport/kafka` - Kafka consumer.

## Локальный Запуск

```shell
go run ./cmd -c config.env
```

Для полноценного запуска нужны доступные Kafka и MongoDB.

## Тесты

Unit-тесты без интеграции:

```shell
make test
```

Все тесты:

```shell
make test-all
```

Интеграционные тесты MongoDB:

```shell
make test-integration
```

Интеграционные тесты автоматически пропускаются, если Docker недоступен. Также их можно явно пропустить:

```shell
make test-skip-integration-mongo
```

## Сборка

```shell
make build
```
