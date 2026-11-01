# Gym

Скелет сервиса на Go: Gin + MySQL 8 + Redis (asynq) + Slog + Viper + Cobra + goose-миграции + Docker Compose. DI ручной.

## Быстрый старт (Docker)

```bash
cp .env.example .env
docker compose up -d --build   # или make up
```

Поднимаются: MySQL 8, Redis 7, `migrate` (одноразовый, применяет миграции), `app` (API), `worker` (asynq + scheduler). Среда контейнеров берётся из `.env` (`GYM_ENV`, дефолт `prod`): при `dev` — text-логи в stdout и debug-уровень; при `prod` — json в stdout + `/var/log/gym/app.log` (volume `logs_data`, ротация lumberjack).

Проверка:

```bash
curl localhost:8080/healthz
curl localhost:8080/ready
curl -X POST localhost:8080/api/v1/tasks/example -H 'Content-Type: application/json' -d '{"message":"hello"}'
```

Веб-UI очередей asynq (asynqmon):

```bash
docker compose --profile ui up -d asynqmon   # или make ui
# http://localhost:8081
```

## Локальная разработка (без Docker)

Нужны запущенные MySQL и Redis (можно только их: `docker compose up -d mysql redis`).

```bash
make run            # API (GYM_ENV=dev по умолчанию)
make worker         # фоновый воркер
make migrate-up     # применить миграции (goose, зашиты в бинарник)
make migrate-create name=add_sessions   # создать миграцию
```

## Тесты

Отдельная тестовая инфраструктура, чтобы не трогать dev-данные: `mysql-test` (:3307, база `gym_test`) и `redis-test` (:6380) в compose-профиле `test`.

```bash
make test-db-up     # поднять тестовые mysql/redis
make test           # GYM_ENV=test go test ./...
make test-db-down   # остановить
```

Интеграционные тесты: `internal/testutil.Start(t)` грузит `config/test.yaml`, применяет миграции и собирает приложение тем же DI, что и прод. Примеры: `handler/health_test.go`, `tasks/example_test.go`.

## Конфигурация

**Среда** обязательна и выбирается по приоритету: флаг `--env` > переменная `GYM_ENV` > ошибка запуска.
Разрешённые значения: `dev` | `prod` | `test`. Файл конфига: `config/<env>.yaml` (или `--config path`).

Значения: `GYM_* env` > yaml > дефолты в коде (12-factor):

| Env                        | Config key                  |
|----------------------------|-----------------------------|
| `GYM_HTTP_PORT`            | `http.port`                 |
| `GYM_MYSQL_HOST`           | `mysql.host`                |
| `GYM_REDIS_ADDR`           | `redis.addr`                |
| `GYM_LOGGER_LEVEL`         | `logger.level`              |
| `GYM_LOGGER_FORMAT`        | `logger.format`             |
| `GYM_LOGGER_OUTPUTS`       | `logger.outputs` (через `,`)|
| `GYM_LOGGER_FILE_PATH`     | `logger.file.path`          |

Профили логов: dev — text → stdout; prod — json → stdout + файл `/var/log/gym/app.log` с ротацией.

## CLI

```
gym serve                  # HTTP API
gym worker                 # фоновые таски + cron-scheduler
gym migrate up|down|status|create NAME
```

Все команды (кроме `migrate create`) требуют GYM_ENV.

## Фоновые таски

- `tasks:example` — пример таска с JSON-пейлоадом (ставится через `POST /api/v1/tasks/example`)
- `db:cleanup` — очистка старых записей; запускается cron'ом (`0 3 * * *`, очередь `maintenance`) или вручную `POST /api/v1/tasks/db-cleanup`
- Очереди с приоритетами: `critical` > `default` > `maintenance`
- Ретраи с backoff, персистентность в Redis

## Структура

```
cmd/gym/            точка входа
internal/cmd/       Cobra-команды (serve, worker, migrate)
internal/app/       ручной DI
internal/config/    Viper (окружения dev|prod|test, env overrides)
internal/logger/    Slog + lumberjack (stdout/файл)
internal/database/  MySQL (sqlx), Redis
internal/server/    Gin-сервер, graceful shutdown
internal/transport/ handlers + middleware
internal/tasks/     asynq: client, worker, scheduler, handlers
internal/testutil/  хелперы интеграционных тестов
config/             dev.yaml, prod.yaml, test.yaml
migrations/         goose-миграции (embed)
```

## Make-таргеты

`make up|down|logs|ps|ui` — Docker; `make run|worker|build|vet|tidy|test` — локально; `make test-db-up|test-db-down` — тест-инфраструктура; `make migrate-*` — миграции.
