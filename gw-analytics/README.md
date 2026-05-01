# gw-analytics

`gw-analytics` - фоновый сервис аналитики. Он читает события операций из Kafka, валидирует их, обрабатывает микробатчами и передает дальше в слой хранения.

Слой Elasticsearch пока намеренно не реализован. Мы оставили для него интерфейс storage, чтобы отдельно спокойно разобрать Elastic, индекс, документ, идемпотентность и агрегации.

## Задача сервиса

По ТЗ сервис должен получать события от `gw-currency-wallet` через брокер сообщений и считать аналитику:

- количество событий по типам операций;
- количество событий по статусам;
- latency от создания события до доставки;
- частоту ошибок и повторных обработок;
- агрегации по периодам: 1 минута, 5 минут, 1 час, 1 день, 1 неделя;
- доставку `at-least-once` с идемпотентностью.

## Текущий статус

Сейчас готов каркас:

- загрузка конфига через `viper`;
- логгер такой же, как в остальных сервисах;
- Kafka consumer с batch-обработкой;
- контракт события `OperationEvent`;
- валидация события в domain/usecase;
- интерфейс storage под будущий Elasticsearch;
- unit-тесты через `suite`;
- автогенерация моков через `mockery`;
- Dockerfile и entrypoint.

Пока не реализовано:

- подключение к Elasticsearch;
- сохранение событий в Elasticsearch;
- создание индекса и mapping;
- запросы/агрегации к Elasticsearch;
- docker-compose секция для `gw-analytics` и Elasticsearch.

## Поток данных

```text
gw-currency-wallet -> Kafka topic wallet.operations -> gw-analytics -> Elasticsearch
```

`gw-analytics` не принимает HTTP или gRPC запросы. Это отдельный Kafka consumer.

## Контракт события

Событие приходит в Kafka как JSON:

```json
{
  "event_id": "uuid",
  "user_id": 1,
  "operation_type": "DEPOSIT",
  "status": "SUCCESS",
  "currency": "RUB",
  "amount_minor": 5000000,
  "amount_rub_minor": 5000000,
  "created_at": "2026-04-30T12:00:00Z",
  "error": ""
}
```

Поля:

- `event_id` - уникальный идентификатор события, нужен для идемпотентности.
- `user_id` - идентификатор пользователя в wallet-сервисе.
- `operation_type` - тип операции: `DEPOSIT`, `WITHDRAW`, `EXCHANGE`.
- `status` - результат операции: `SUCCESS` или `FAILED`.
- `currency` - исходная валюта операции.
- `amount_minor` - сумма операции в minor units.
- `amount_rub_minor` - сумма операции в RUB minor.
- `created_at` - время создания события на стороне wallet.
- `error` - текст ошибки для неуспешной операции.

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
после успешной обработки закоммитить Kafka offset
```

Битый JSON не валит весь batch: сообщение логируется как decode error и пропускается.

Если usecase/storage вернул ошибку, offset не коммитится. Kafka сможет доставить batch повторно, поэтому storage обязан быть идемпотентным.

## Идемпотентность

Kafka дает режим `at-least-once`: сообщение может прийти повторно.

План для Elasticsearch:

- использовать `event_id` как `_id` документа;
- повторная запись события с тем же `event_id` не должна создавать дубль;
- commit offset делать только после успешной записи batch;
- ошибки записи логировать и возвращать наверх, чтобы batch был обработан повторно.

## Конфигурация

Основные переменные:

```env
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=wallet.operations
KAFKA_GROUP_ID=gw-analytics
KAFKA_MIN_BYTES=1
KAFKA_MAX_BYTES=10485760
KAFKA_MAX_WAIT_MS=500
KAFKA_BATCH_SIZE=128
KAFKA_BATCH_WAIT_MS=50
LOG_LEVEL=debug
```

В Docker Compose сервис позже будет обращаться к Kafka и Elasticsearch по именам контейнеров.

## Локальный запуск

```shell
go run ./cmd -c config.env
```

На текущем этапе запуск приложения завершится ошибкой `analytics storage is not implemented`. Это ожидаемо: storage для Elasticsearch будет добавлен отдельным шагом.

## Docker

В сервисе уже есть Dockerfile и простой `entrypoint.sh`.

Контейнер можно собрать, но полноценно запускать сервис в compose будем после того, как добавим Elasticsearch storage и зависимый контейнер Elasticsearch.

## Тесты и моки

Моки генерируются через `mockery` по конфигу `.mockery.yaml`.

```shell
make mocks
make test
```

`make test` сначала перегенерирует моки, затем запускает unit-тесты.

Интеграционных тестов пока нет, потому что слой Elasticsearch еще не написан. Когда добавим storage, сделаем интеграционные тесты со skip-флагом и автоматическим пропуском, если Docker недоступен.

## Линтинг и сборка

```shell
make lint
make build
```

Линтер настроен через `.golangci.yml`; тестовые файлы линтером не проверяются.
