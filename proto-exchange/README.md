# proto-exchange

`proto-exchange` содержит gRPC-контракт сервиса курсов валют. Этот модуль нужен как общий источник правды между `gw-exchanger` и `gw-currency-wallet`.

## Назначение

Контракт описывает сервис `ExchangeService`, через который wallet получает курсы валют из exchanger:

- `GetExchangeRates` возвращает все доступные курсы.
- `GetExchangeRateForCurrency` возвращает курс из одной валюты в другую.

Поддерживаемые валюты на уровне бизнес-логики сервисов: `USD`, `RUB`, `EUR`.

## Структура

```text
proto-exchange/
├── exchange/
│   ├── exchange.proto
│   ├── exchange.pb.go
│   └── exchange_grpc.pb.go
└── go.mod
```

## Генерация Go-кода

Из корня `proto-exchange`:

```shell
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  exchange/exchange.proto
```

Для генерации нужны плагины:

```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Использование

В сервисах контракт импортируется как Go-модуль:

```go
import exchangegrpc "github.com/paxaf/itkFinal/proto-exchange/exchange"
```

`gw-exchanger` реализует серверную сторону контракта, а `gw-currency-wallet` использует сгенерированный gRPC-клиент.

## Доступы и использование в compose

Сам модуль не запускается как сервис. Его используют:

- `gw-exchanger` как gRPC server;
- `gw-currency-wallet` как gRPC client.

В общем compose exchanger слушает:

```text
gw-exchanger:50051
```

С хоста gRPC endpoint доступен как:

```text
localhost:50051
```

Если compose запущен внутри VM, вместо `localhost` нужно использовать IP этой VM.

## Статус перед сдачей

- Контракт вынесен в отдельный Go-модуль.
- Сгенерированные Go-файлы лежат рядом с `exchange.proto`.
- `gw-exchanger` и `gw-currency-wallet` используют один и тот же контракт.
