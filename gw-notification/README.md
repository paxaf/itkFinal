# gw-notification

`gw-notification` - фоновый Kafka consumer для обработки крупных денежных операций. Он читает события из Kafka, валидирует их и сохраняет историю в MongoDB.

## Что делает сервис

- Читает JSON-события из Kafka topic `wallet.large-operations`.
- Обрабатывает сообщения микробатчами.
- Валидирует каждое событие на уровне usecase.
- Сохраняет валидные события в MongoDB.
- Создаёт уникальный индекс по `event_id`, чтобы повторная доставка Kafka не создавала дубли.
- Коммитит Kafka offset только после успешной обработки batch.
- Логирует ошибки декодирования, ошибки обработки, результат batch и ошибки commit.
- Корректно завершается по `SIGINT` и `SIGTERM`.

## Поток данных

```text
gw-currency-wallet -> Kafka wallet.large-operations -> gw-notification -> MongoDB
```

Сервис не принимает HTTP или gRPC запросы. Это отдельный фоновый consumer.

## Контракт события

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
- `user_id` - идентификатор пользователя из wallet.
- `operation_type` - тип операции: `DEPOSIT`, `WITHDRAW`, `EXCHANGE`.
- `currency` - исходная валюта операции.
- `amount_minor` - сумма операции в minor units.
- `amount_rub_minor` - сумма операции в RUB minor.
- `created_at` - время создания события на стороне wallet.

## Надёжность

Kafka даёт доставку как минимум один раз. Поэтому сервис должен быть идемпотентным.

Идемпотентность сделана через уникальный индекс MongoDB по `event_id`. Если Kafka повторно доставит уже сохранённое событие, MongoDB вернёт duplicate key error, а storage-слой обработает это как успешный результат.

Если сохранение batch в MongoDB завершилось ошибкой, Kafka offset не коммитится. Это позволяет обработать batch повторно после восстановления сервиса.

## Batch-обработка

Consumer работает так:

```text
получить первое сообщение
        |
        v
добрать batch до KAFKA_BATCH_SIZE или до KAFKA_BATCH_WAIT_MS
        |
        v
декодировать JSON
        |
        v
валидные события отправить в usecase
        |
        v
после успешного сохранения закоммитить Kafka offset
```

Битый JSON не валит весь batch: сообщение логируется как decode error и пропускается.

## Конфигурация

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

В Docker Compose сервис обращается к зависимостям по именам контейнеров:

- Kafka: `kafka:9092`
- MongoDB: `notification-mongo:27017`

## Локальный запуск

```shell
go run ./cmd -c config.env
```

Для запуска нужны доступные Kafka и MongoDB.

## Docker

Из корня репозитория:

```shell
docker compose up --build gw-notification
```

У сервиса есть простой `entrypoint.sh`, который запускает переданную команду. Миграции не нужны, потому что MongoDB индекс создаётся приложением при подключении.

## Тесты и моки

Моки генерируются через `mockery` по конфигу `.mockery.yaml`.

```shell
make mocks
make test
```

`make test` сначала перегенерирует моки, затем запускает unit-тесты без интеграционных.

Интеграционные тесты MongoDB:

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