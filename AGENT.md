# AGENT.md

Контекст проекта Gym для AI-агента. Коротко, дополняем по мере развития.

## Стек

- Go 1.27.3 (`go.mod`; локально может быть старше — toolchain скачается автоматически)
- HTTP: Gin · БД: MySQL 8 + sqlx (без ORM) · Кэш/очередь: Redis 7 + asynq
- Логи: stdlib Slog · Конфиг: Viper (YAML + env) · CLI: Cobra (`serve`, `worker`, `migrate`)
- Миграции: goose v3, SQL embed в бинарник · Infra: Docker, Docker Compose
- DI ручной (`internal/app/app.go`), фреймворков внедрения нет

## Правила

- **Git**: никаких операций (commit / push / amend и т.п.) без явного разрешения пользователя
- Комментарии в коде — только по просьбе
- Следовать существующим паттернам слоёв и неймингу

## Команды

```bash
make up            # docker compose up -d --build (mysql, redis, migrate, app, worker)
make ui            # + asynqmon (веб-UI очередей, :8081, профиль "ui")
make down / logs / ps
make run / worker  # локально (нужны mysql+redis, можно docker compose up -d mysql redis)
make migrate-up / migrate-down / migrate-status / migrate-create name=имя
go build ./... && go vet ./...   # проверка перед завершением работы
```

Порты: API :8080, asynqmon :8081, mysql :3306 (gym/gym), redis :6379.

## Паттерны добавления фич

**Новый эндпоинт:**
1. Handler в `internal/transport/http/handler/`
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
4. Значение в `config/local.yaml`

**Миграция:** `make migrate-create name=имя` → SQL-файл в `migrations/` (up и down в одном файле, `-- +goose Up` / `-- +goose Down`). embed.FS — новые файлы подхватываются после пересборки бинарника.

## Слой доступа к данным

- MySQL: sqlx, DSN собирается в `config.MySQL.DSN()` (уже с `parseTime`, `loc=UTC`, `multiStatements` для goose)
- Запросы — сырой SQL в репозиториях/handler'ах тасков, скан в структуры через sqlx

## Известные грабли

- **Viper**: `AutomaticEnv` + `Unmarshal` не видит env для ключей без дефолта/BindEnv → все скалярные ключи перечислены в `envKeys` (`internal/config/config.go`)
- **goose v3.29**: `MigrationStatus.Source` — это `*goose.Source` с полями `Version`, `Path`
- **asynq v0.26**: `asynq.Logger` требует метод `Fatal`
- В контейнерах нет `config/local.yaml` — всё приходит через `GYM_*` env (compose-якорь `x-app-env`)
- Graceful shutdown: и сервер, и worker слушают SIGTERM/SIGINT через `signal.NotifyContext`
