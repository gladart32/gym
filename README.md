# Gym

Трекер тренировок. Монорепозиторий: бэкенд на Go + SPA на Vue 3, мобильное приложение — позже (Capacitor, тот же фронтенд-код).

## Структура

```
backend/            Go-сервис: Gin, MySQL 8, Redis 7 (asynq), Slog, Viper, Cobra, goose
frontend/           Vue 3 SPA: Vite, Tailwind CSS v4, shadcn-vue, vue-i18n (ru/en), Pinia
backend/media/      GIF и миниатюры упражнений (make media-sync, в git не попадают)
data/               каталог упражнений: переводы ru (в git) + клон датасета (make dataset-fetch, не в git)
docs/               описание схемы БД (DB.md) и флоу авторизации (AUTH.md)
docker-compose.yml  инфраструктура (build-контекст ./backend; nginx — точка входа :80)
Makefile            единая точка входа для всех команд
var/                логи контейнеров (bind mount, не в git)
```

Контекст для AI-агентов: `AGENT.md` в корне и в каждой из папок. Описание схемы БД и флоу авторизации — в `docs/` (`DB.md`, `AUTH.md`); план реализации авторизации — `AUTH.md` в корне.

## Быстрый старт (Docker)

```bash
cp .env.example .env
make dataset-fetch  # клон датасета упражнений (GIF/миниатюры, ~136 МБ)
make media-sync     # скопировать GIF/миниатюры в backend/media
make up             # front-build + mysql, redis, migrate, app, worker, nginx
make seed-catalog   # залить каталог упражнений (1324 шт., ru/en) — нужен запущенный mysql
```

Вход в приложение: http://localhost (nginx раздаёт SPA, `/api` и `/media` — проксирует).

Проверка:

```bash
curl localhost/healthz
curl localhost/ready
curl -X POST localhost/api/v1/tasks/example -H 'Content-Type: application/json' -d '{"message":"hello"}'
```

Веб-UI очередей asynq: `make ui` → http://localhost:8081.

## Frontend

```bash
make front-install   # npm install
make front-dev       # dev-сервер :5173, /api проксируется на :8080
make front-build     # production-сборка в frontend/dist
```

## Локальная разработка (Go без Docker)

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

Интеграционные тесты: `backend/internal/testutil.Start(t)` грузит `config/test.yaml`, применяет миграции и собирает приложение тем же DI, что и прод. Примеры: `handler/health_test.go`, `tasks/example_test.go`.

## Конфигурация (бэкенд)

**Среда** обязательна и выбирается по приоритету: флаг `--env` > переменная `GYM_ENV` > ошибка запуска.
Разрешённые значения: `dev` | `prod` | `test`. Файл конфига: `backend/config/<env>.yaml` (или `--config path`).

Значения: `GYM_* env` > yaml > дефолты в коде (12-factor):

| Env                     | Config key                   |
|-------------------------|------------------------------|
| `GYM_HTTP_PORT`         | `http.port`                  |
| `GYM_MYSQL_HOST`        | `mysql.host`                 |
| `GYM_REDIS_ADDR`        | `redis.addr`                 |
| `GYM_LOGGER_LEVEL`      | `logger.level`               |
| `GYM_LOGGER_FORMAT`     | `logger.format`              |
| `GYM_LOGGER_OUTPUTS`    | `logger.outputs` (через `,`) |
| `GYM_LOGGER_FILE_PATH`  | `logger.file.path`           |

Логи контейнеров пишутся в `var/log/gym/` на хосте (bind mount, настраивается `LOGS_DIR` в `.env`): `app.log`, `worker.log`, `migrate.log` — раздельно на процесс, ротация lumberjack.

## CLI (бэкенд)

```
gym serve                                  # HTTP API
gym worker                                 # фоновые таски + cron-scheduler
gym migrate up|down|status|create NAME
```

Все команды (кроме `migrate create`) требуют GYM_ENV.

## Фоновые таски

- `tasks:example` — пример таска с JSON-пейлоадом (ставится через `POST /api/v1/tasks/example`)
- `db:cleanup` — очистка старых записей; cron `0 3 * * *` (очередь `maintenance`) или вручную `POST /api/v1/tasks/db-cleanup`
- Очереди с приоритетами: `critical` > `default` > `maintenance`
- Ретраи с backoff, персистентность в Redis

## Make-таргеты

Docker: `make up|down|start|stop|logs|ps|ui` · Go: `make run|worker|build|test|vet|tidy|migrate-*` ·
Данные каталога: `make dataset-fetch|media-sync|seed-catalog` ·
Тест-инфраструктура: `make test-db-up|test-db-down` · Frontend: `make front-install|front-dev|front-build|front-lint|front-format`
