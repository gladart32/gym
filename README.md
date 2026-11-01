# Gym

Скелет сервиса на Go: Gin + MySQL 8 + Redis (asynq) + Slog + Viper + Cobra + goose-миграции + Docker Compose. DI ручной.

## Быстрый старт (Docker)

```bash
cp .env.example .env
docker compose up -d --build   # или make up
```

Поднимаются: MySQL 8, Redis 7, `migrate` (одноразовый, применяет миграции), `app` (API), `worker` (asynq + scheduler).

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
make run            # API
make worker         # фоновый воркер
make migrate-up     # применить миграции (goose, зашиты в бинарник)
make migrate-create name=add_sessions   # создать миграцию
```

## Конфигурация

`config/local.yaml` — базовые значения; переменные окружения с префиксом `GYM_` перекрывают (12-factor):

| Env                  | Config key                |
|----------------------|---------------------------|
| `GYM_HTTP_PORT`      | `http.port`               |
| `GYM_MYSQL_HOST`     | `mysql.host`              |
| `GYM_REDIS_ADDR`     | `redis.addr`              |
| `GYM_LOGGER_LEVEL`   | `logger.level`            |
| `GYM_LOGGER_FORMAT`  | `logger.format`           |

Файл конфигурации опционален — все ключи имеют дефолты. Путь к файлу: `--config path/to.yaml`.

## CLI

```
gym serve                  # HTTP API
gym worker                 # фоновые таски + cron-scheduler
gym migrate up|down|status|create NAME
```

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
internal/config/    Viper (YAML + env)
internal/logger/    Slog
internal/database/  MySQL (sqlx), Redis
internal/server/    Gin-сервер, graceful shutdown
internal/transport/ handlers + middleware
internal/tasks/     asynq: client, worker, scheduler, handlers
migrations/         goose-миграции (embed)
```

## Make-таргеты

`make up|down|logs|ps|ui` — Docker; `make run|worker|build|vet|tidy` — локально; `make migrate-*` — миграции.
