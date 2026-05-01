# gw-analytics

`gw-analytics` - фоновый сервис аналитики. Он читает события операций из Kafka, валидирует их, обрабатывает микробатчами и сохраняет в Elasticsearch.

Kibana используется как интерфейс для просмотра документов и построения агрегаций поверх индекса Elasticsearch.

## Задача сервиса

По ТЗ сервис должен получать события от `gw-currency-wallet` через брокер сообщений и считать аналитику:

- количество событий по типам операций;
- количество событий по статусам;
- latency от создания события до доставки;
- частоту ошибок и повторных обработок;
- агрегации по периодам: 1 минута, 5 минут, 1 час, 1 день, 1 неделя;
- доставку `at-least-once` с идемпотентностью.

## Текущий статус

Сервис готов к запуску в общем `docker-compose`:

- загрузка конфига через `viper`;
- логгер такой же, как в остальных сервисах;
- Kafka consumer с batch-обработкой;
- контракт события `OperationEvent`;
- валидация события в domain/usecase;
- Elasticsearch storage с bulk-записью событий;
- идемпотентность через `event_id` как `_id` документа;
- подключение Elasticsearch storage в `app`;
- unit-тесты через `suite`;
- интеграционные тесты Elasticsearch через testcontainers;
- автогенерация моков через `mockery`;
- Dockerfile и entrypoint.

Агрегации по типам, статусам, latency, ошибкам и периодам делаются на стороне Kibana через Data View, Lens и Dashboard.

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

Реализация в Elasticsearch:

- использовать `event_id` как `_id` документа;
- повторная запись события с тем же `event_id` не создаёт дубль;
- повторная доставка увеличивает `delivery_count` и `duplicate_count`;
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
ELASTIC_ADDRESSES=http://localhost:9200
ELASTIC_USERNAME=
ELASTIC_PASSWORD=
ELASTIC_INDEX=wallet_operations
LOG_LEVEL=debug
```

В Docker Compose сервис обращается к Kafka и Elasticsearch по именам контейнеров:

- Kafka: `kafka:9092`
- Elasticsearch: `http://elasticsearch:9200`
- Kibana: `http://localhost:5601`

## Локальный запуск

```shell
go run ./cmd -c config.env
```

Для локального запуска нужны доступные Kafka и Elasticsearch. Внутри общего compose эти зависимости уже настроены.

## Docker

В сервисе есть Dockerfile и простой `entrypoint.sh`.

Из корня репозитория:

```shell
docker compose up --build gw-analytics kibana
```

В общем compose также поднимаются Kafka и Elasticsearch как зависимости.

Для запуска всего стенда:

```shell
docker compose up --build
```

## Kibana

После запуска compose открой Kibana:

```text
http://localhost:5601
```

В Kibana нужно создать Data View:

```text
Index pattern: wallet_operations
Timestamp field: created_at
```

После этого данные будут доступны в Discover, Lens и Dashboard.

## Тесты и моки

Моки генерируются через `mockery` по конфигу `.mockery.yaml`.

```shell
make mocks
make test
```

`make test` сначала перегенерирует моки, затем запускает unit-тесты.

Интеграционные тесты Elasticsearch:

```shell
make test-integration
```

Если Docker недоступен, интеграционные тесты автоматически пропускаются. Для явного пропуска:

```shell
make test-skip-integration-elastic
```

## Линтинг и сборка

```shell
make lint
make build
```

Линтер настроен через `.golangci.yml`; тестовые файлы линтером не проверяются.

## Доступы в общем compose

Сервис не открывает внешний HTTP или gRPC порт. Он работает как фоновый Kafka consumer внутри docker-сети.

Kafka topic:

```text
wallet.operations
```

Elasticsearch доступен с хоста:

```text
http://localhost:9200
```

Kibana доступна с хоста:

```text
http://localhost:5601
```

Если compose запущен внутри VM, вместо `localhost` нужно использовать IP этой VM:

```text
http://<vm-ip>:5601
```

Внутри docker-сети сервис использует:

- Kafka: `kafka:9092`;
- Elasticsearch: `http://elasticsearch:9200`;
- индекс Elasticsearch: `wallet_operations`.

## Агрегации по периодам

Требование по получению данных за периоды `1 минута`, `5 минут`, `1 час`, `1 день`, `1 неделя` выполняется на стороне Kibana поверх индекса Elasticsearch.

Сервис сохраняет каждое событие с полями:

- `created_at` - время создания события в wallet;
- `delivered_at` - время сохранения события в analytics;
- `latency_ms` - задержка между созданием и доставкой;
- `operation_type` - тип операции;
- `status` - статус операции;
- `error` - текст ошибки для неуспешных операций.

В Kibana создаётся Data View `wallet_operations` с timestamp field `created_at`. После этого в Lens или Dashboard можно строить:

- количество событий по `operation_type`;
- количество событий по `status`;
- среднюю, минимальную, максимальную и percentile latency по `latency_ms`;
- частоту ошибок через фильтр `status: FAILED`;
- агрегации по времени через Date histogram с интервалами `1m`, `5m`, `1h`, `1d`, `1w`.

Такой вариант оставляет сервис простым consumer/storage-слоем, а аналитические срезы переносит в инструмент, который для этого предназначен.

## Проверка данных

Количество документов в индексе можно проверить так:

```shell
curl http://localhost:9200/wallet_operations/_count
```

Пример запроса latency-статистики:

```shell
curl -X POST http://localhost:9200/wallet_operations/_search \
  -H "Content-Type: application/json" \
  -d '{"size":0,"aggs":{"latency":{"stats":{"field":"latency_ms"}}}}'
```

## Статус перед сдачей

- Сервис читает topic всех операций `wallet.operations`.
- Batch-обработка настроена через `KAFKA_BATCH_SIZE` и `KAFKA_BATCH_WAIT_MS`.
- Offset коммитится после успешной обработки batch.
- Идемпотентность реализована через `event_id` как `_id` документа Elasticsearch.
- Повторная доставка не создаёт дубль и увеличивает счётчики доставки.
- Unit-тесты и интеграционные Elasticsearch-тесты проходят.
- Моки генерируются через `mockery`.
- Линтер и сборка проходят.
