# itkFinal

Репозиторий содержит набор микросервисов для кошелька, обмена валют, уведомлений о крупных операциях и аналитики событий.

## Сервисы

- `gw-currency-wallet` - REST API кошелька, регистрация, авторизация, балансы, пополнение, вывод, обмен валют и публикация событий в Kafka.
- `gw-exchanger` - gRPC-сервис курсов валют, хранит курсы в PostgreSQL и отдаёт их по контракту `proto-exchange`.
- `gw-notification` - Kafka consumer крупных операций, сохраняет историю крупных операций в MongoDB.
- `gw-analytics` - Kafka consumer всех операций, сохраняет события в Elasticsearch для анализа через Kibana.
- `proto-exchange` - общий gRPC-контракт между wallet и exchanger.

## Общий запуск

Из корня репозитория:

```shell
docker compose up --build
```

Для чистого перезапуска стенда с удалением старых данных:

```shell
docker compose down -v --remove-orphans
docker compose up --build
```

При запуске автоматически поднимаются PostgreSQL, MongoDB, Kafka, Elasticsearch и Kibana. Миграции PostgreSQL выполняются в entrypoint контейнеров `gw-exchanger` и `gw-currency-wallet` через `goose`.

## Доступы локального стенда

Если compose запущен на локальной машине, используй `localhost`. Если compose запущен внутри VM, вместо `localhost` используй IP этой VM.

| Компонент | Адрес с хоста | Назначение |
| --- | --- | --- |
| Wallet REST API | `http://localhost:8080` | Основное API клиента |
| Wallet Swagger | `http://localhost:8080/swagger/index.html` | Документация REST API |
| Exchanger gRPC | `localhost:50051` | gRPC API курсов валют |
| Kibana | `http://localhost:5601` | Просмотр аналитики |
| Elasticsearch | `http://localhost:9200` | Хранилище событий аналитики |
| Kafka | `localhost:9092` | Брокер сообщений |
| Exchanger PostgreSQL | `localhost:5433` | База курсов валют |
| Wallet PostgreSQL | `localhost:5434` | База пользователей и балансов |
| Notification MongoDB | `localhost:27018` | История крупных операций |

Внутри docker-сети сервисы обращаются друг к другу по именам контейнеров: `gw-exchanger`, `wallet-db`, `exchanger-db`, `kafka`, `notification-mongo`, `elasticsearch`.

## Учётные данные dev-стенда

Эти значения используются только для локального compose-стенда:

| Компонент | Пользователь | Пароль | База |
| --- | --- | --- | --- |
| Exchanger PostgreSQL | `postgres` | `postgres` | `exchange` |
| Wallet PostgreSQL | `postgres` | `postgres` | `wallet` |
| MongoDB | `mongo` | `mongo` | `notification` |
| Elasticsearch | без авторизации | без авторизации | `wallet_operations` |

JWT secret для локального стенда: `local-dev-secret`.

## Kafka topics

Топики создаются контейнером `kafka-init` до старта consumers:

- `wallet.operations` - все операции wallet для аналитики.
- `wallet.large-operations` - только крупные операции от 30 000 RUB для notification.

## Основной e2e-сценарий

1. Проверить `GET /api/v1/health`.
2. Зарегистрировать пользователя через `POST /api/v1/register`.
3. Авторизоваться через `POST /api/v1/login` и получить JWT.
4. Проверить баланс через `GET /api/v1/balance`.
5. Получить курсы через `GET /api/v1/exchange/rates`.
6. Пополнить баланс через `POST /api/v1/wallet/deposit`.
7. Выполнить обмен через `POST /api/v1/exchange`.
8. Выполнить крупную операцию от `30000 RUB`, чтобы событие попало в MongoDB и Elasticsearch.
9. Открыть Kibana и проверить события в индексе `wallet_operations`.

## Проверка качества

Для каждого сервиса доступны стандартные команды:

```shell
make install-tools
make test
make test-integration
make lint
make build
```

`make test` генерирует моки через `mockery` и запускает unit-тесты без интеграционных. Интеграционные тесты используют testcontainers и автоматически пропускаются, если Docker недоступен.

На финальной проверке были прогнаны unit-тесты, интеграционные тесты, линтеры и сборка для сервисов `gw-exchanger`, `gw-currency-wallet`, `gw-notification`, `gw-analytics`.

## Покрытие требований

- Безопасность: приватные ручки wallet защищены JWT.
- Производительность: клиентские REST-запросы в e2e-сценарии укладываются в 200 мс для основных операций.
- Логирование: сервисы используют единый структурированный logger и логируют запуск, ошибки и обработку операций.
- Тестирование: основные функции покрыты unit-тестами, storage-слои проверяются интеграционными тестами.
- Документация: REST API wallet доступен через Swagger, сервисы описаны в README.
- Надёжность Kafka: consumers коммитят offset после успешной обработки batch, storage-слои реализуют идемпотентность.
- Аналитика: события операций сохраняются в Elasticsearch, агрегации строятся через Kibana.