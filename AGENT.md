# AGENT.md

Контекст проекта Gym для AI-агента. Коротко, дополняем по мере развития.

## Стек

- Go 1.27.3 (`go.mod`; локально может быть старше — toolchain скачается автоматически)
- HTTP: Gin · БД: MySQL 8 + sqlx (без ORM) · Кэш/очередь: Redis 7 + asynq
- Логи: stdlib Slog (stdout / файл с ротацией lumberjack) · Конфиг: Viper (YAML + env) · CLI: Cobra (`serve`, `worker`, `migrate`)
- Миграции: goose v3, SQL embed в бинарник · Infra: Docker, Docker Compose
- DI ручной (`internal/app/app.go`), фреймворков внедрения нет

## Правила

- **Git**: никаких операций (commit / push / amend и т.п.) без явного разрешения пользователя
- Комментарии в коде — только по просьбе
- Следовать существующим паттернам слоёв и неймингу

## Окружения и конфигурация

Среды: **dev | prod | test** (белый список, `config.Envs`). Приоритет: `--env` флаг > `GYM_ENV` > **ошибка запуска** (fail-fast, без дефолта).

- Файлы: `config/{dev,prod,test}.yaml` — выбирается `config/<env>.yaml`, если не задан `--config`
- Значения: `GYM_* env` > yaml > дефолты в `setDefaults`
- Makefile: `export GYM_ENV ?= dev` — локальные таргеты работают без ручных действий
- Compose-контейнеры: `GYM_ENV: ${GYM_ENV:-prod}` в якоре `x-app-env` — среда стека берётся из `.env` (дефолт `prod`, если не задана). Образы содержат `config/` (лежит в `/config`)
- `gym migrate create` — единственная команда без GYM_ENV и конфига
- Первая строка лога при старте показывает env/format/outputs — проверять при отладке конфига

Логи: `logger.outputs` (stdout/file в любом сочетании) — во всех env включены оба вывода. dev/test пишут в относительный `logs/app.log` (локально; папку логгер создаёт сам), prod — `/var/log/gym/app.log`. В compose путь подменяется env `GYM_LOGGER_FILE_PATH`, а контейнерные логи монтируются на хост **bind mount'ом `./var/log/gym`** (настраивается `LOGS_DIR` в `.env`) — файлы видны в IDE. Файлы раздельные: **app.log / worker.log / migrate.log** (один lumberjack-инстанс на процесс: два процесса с ротацией одного файла = гонка). `make worker` локально пишет `logs/worker.log`.

## Команды

```bash
make up            # docker compose up -d --build (mysql, redis, migrate, app, worker; GYM_ENV=prod внутри)
make start / stop  # запуск/остановка уже созданных контейнеров (без пересборки и удаления; первый раз — make up)
make ui            # + asynqmon (веб-UI очередей, :8081, профиль "ui")
make down / logs / ps
make run / worker  # локально (GYM_ENV=dev; нужны mysql+redis: docker compose up -d mysql redis)
make test-db-up    # тест-инфраструктура: mysql-test :3307 (gym_test), redis-test :6380 (профиль "test")
make test          # GYM_ENV=test go test ./...
make migrate-up / migrate-down / migrate-status / migrate-create name=имя
go build ./... && go vet ./...   # проверка перед завершением работы
```

Порты: API :8080, asynqmon :8081, mysql :3306 (gym/gym), redis :6379, mysql-test :3307 (gym_test/gym_test), redis-test :6380.

## Паттерны добавления фич

**Новый эндпоинт:**
1. Handler в `internal/transport/http/handler/` (ответ — только через `response.Success/Error`, см. «Формат ответов API»)
2. Роут в `internal/server/router.go` (группа `/api/v1`)
3. Зависимости конструировать в `internal/app/app.go` и передавать явно

**Новая фоновая задача (asynq):**
1. Тип таска в `internal/tasks/types.go` + payload-структура
2. Метод enqueue в `internal/tasks/client.go`
3. Handler `ProcessTask` в `internal/tasks/<name>.go`
4. Регистрация в mux в `internal/tasks/worker.go`
5. Периодическая — расписание в `internal/tasks/scheduler.go` (scheduler строго в одном экземпляре, cron-синтаксис)

**Новое поле конфигурации:**
1. Поле в struct + `mapstructure` тег в `internal/config/config.go`
2. Дефолт в `setDefaults` (если уместен)
3. **Обязательно** ключ в `envKeys` — без BindEnv env-переменная не подхватится (см. грабли)
4. Значения в `config/{dev,prod}.yaml` (и `test.yaml`, если отличается)

**Миграция:** `make migrate-create name=имя` → SQL-файл в `migrations/` (up и down в одном файле, `-- +goose Up` / `-- +goose Down`). embed.FS — новые файлы подхватываются после пересборки бинарника.

**Интеграционные тесты:**
1. `make test-db-up` (или через CI-скрипт) — тест-инфраструктура должна быть поднята
2. `testutil.Start(t)` — конфиг `config/test.yaml` (абсолютный путь через `runtime.Caller`), миграции up, полный DI `*app.App`, cleanup автоматом
3. Внешний тест-пакет (`package xxx_test`), если импортируем testutil (иначе цикл: testutil → app → тестируемый пакет)

## Слой доступа к данным

- MySQL: sqlx, DSN собирается в `config.MySQL.DSN()` (уже с `parseTime`, `loc=UTC`, `multiStatements` для goose)
- Запросы — сырой SQL в репозиториях/handler'ах тасков, скан в структуры через sqlx

## Формат ответов API

Единый envelope для всех JSON-ответов. Хелперы: `internal/transport/http/response`.

- Успех: `response.Success(c, code, data)` → `{"status":"success","data":<...>}`; `data` опускается при nil
- Ошибка: `response.Error(c, code, msg)` → `{"status":"error","message":"..."}`; делает Abort (безопасна и в middleware)
- Не-JSON ответы (файлы, редиректы — `c.File`/`c.Data`/`c.Redirect`) пишутся напрямую, минуя хелперы; middleware-обёртку сознательно не используем
- Хелс-чеки (/healthz, /ready) тоже в envelope; /ready при 503 отдаёт только message, детали проверок — в лог

## Известные грабли

- **Viper**: `AutomaticEnv` + `Unmarshal` не видит env для ключей без дефолта/BindEnv → все скалярные ключи перечислены в `envKeys` (`internal/config/config.go`)
- **goose v3.29**: `MigrationStatus.Source` — это `*goose.Source` с полями `Version`, `Path`
- **asynq v0.26**: `asynq.Logger` требует метод `Fatal`
- В контейнерах конфиг лежит в `/config` (скопирован в образ), env всё равно перекрывает
- `asynq.queues` (map) не входит в `envKeys` — меняется только через yaml
- Раздельные лог-файлы на процесс обязательны (см. Логи) — ротация общего файла несколькими процессами приводит к потере данных
- Graceful shutdown: и сервер, и worker слушают SIGTERM/SIGINT через `signal.NotifyContext`; лог-файл закрывается последним в `app.Close()`
