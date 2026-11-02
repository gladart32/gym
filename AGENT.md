# AGENT.md

Корень проекта Gym (монорепозиторий). Контекст для AI-агента.

## Структура

- `backend/` — бэкенд (Go). Подробности — `backend/AGENT.md`
- `frontend/` — фронтенд (Vue 3 SPA). Подробности — `frontend/AGENT.md`
- `docker-compose.yml` + `.env` — инфраструктура всего проекта (build-контекст: `./backend`, Dockerfile в `backend/docker/`)
- `Makefile` — единая точка входа для команд (терминал в корне репо)
- `var/` — логи контейнеров (bind mount, в git не попадает)
- Мобильное приложение — позже: frontend оборачивается в нативную оболочку через Capacitor

## Взаимодействие backend ↔ frontend

- API: REST, JSON, базовый префикс `/api/v1`, сервер на `:8080`
- Формат ответов: envelope `{"status":"success","data":...}` / `{"status":"error","message":"..."}` (см. backend/AGENT.md)
- Локализация: фронт шлёт `Accept-Language` (`ru`/`en`); строки UI — во фронте (`frontend/src/locales/`), строки сущностей и ошибок — на бэкенде (отдаются локализованными)
- Локальная разработка: dev-сервер фронта `:5173` проксирует `/api` → `localhost:8080` (CORS не нужен)

## Команды (из корня репо)

```bash
make up            # docker compose up -d --build (mysql, redis, migrate, app, worker; GYM_ENV=prod внутри)
make start / stop  # запуск/остановка уже созданных контейнеров (первый раз — make up)
make ui            # + asynqmon (веб-UI очередей, :8081, профиль "ui")
make down / logs / ps
make run / worker  # go локально (GYM_ENV=dev; нужны mysql+redis: docker compose up -d mysql redis)
make build / test / vet / tidy
make migrate-up / migrate-down / migrate-status / migrate-create name=имя
make test-db-up    # тест-инфраструктура: mysql-test :3307 (gym_test), redis-test :6380 (профиль "test")
make test
make front-install / front-dev / front-build / front-lint / front-format
```

Порты: API :8080, asynqmon :8081, frontend dev :5173, mysql :3306 (gym/gym), redis :6379, mysql-test :3307 (gym_test/gym_test), redis-test :6380.

## Правила

- **Git**: никаких операций (commit / push / amend и т.п.) без явного разрешения пользователя
- Комментарии в коде — только по просьбе
- Изменения внутри `backend/` и `frontend/` — следуя паттернам из их AGENT.md
