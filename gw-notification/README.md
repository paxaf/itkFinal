# gw-notification

`gw-notification` - сервис обработки уведомлений о крупных денежных переводах.

По ТЗ сервис должен получать сообщения от сервиса кошелька через брокер сообщений, сохранять события в MongoDB, обеспечивать надежную обработку, логирование и graceful shutdown.

## Планируемая структура

- `cmd` - точка входа.
- `internal/app` - сборка приложения.
- `internal/config` - загрузка конфигурации.
- `internal/logger` - логирование.
- `internal/domain` - доменные модели событий.
- `internal/usecase` - бизнес-логика обработки уведомлений.
- `internal/storages` - интерфейсы хранилища.
- `internal/storages/mongo` - реализация MongoDB.
- `internal/transport/kafka` - consumer сообщений из Kafka.

## Локальный запуск

```shell
go run ./cmd -c config.env
```

## Тесты

```shell
make test
```
