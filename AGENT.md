# AGENT.md

Корень проекта Gym (монорепозиторий). Контекст для AI-агента.

## Структура

- `backend/` — бэкенд (Go). Подробности — `backend/AGENT.md`
- `frontend/` — фронтенд (Vue 3 SPA). Подробности — `frontend/AGENT.md`
- `AUTH.md` — план реализации аутентификации (этапы, эндпоинты, решения)
- `docs/` — источник истины по схеме БД (`DB.md`) и флоу авторизации (`AUTH.md`)
- `docker-compose.yml` + `.env` — инфраструктура всего проекта (build-контекст: `./backend`, Dockerfile в `backend/docker/`)
- `data/` — каталог упражнений: переводы ru (в git) + клон датасета exercises-dataset (`make dataset-fetch`, в git не попадает)
- `backend/media/` — GIF/миниатюры упражнений (`make media-sync` из `data/exercises-dataset`, в git не попадают)
- `Makefile` — единая точка входа для команд (терминал в корне репо)
- `var/` — логи контейнеров (bind mount, в git не попадает)
- Мобильное приложение — позже: frontend оборачивается в нативную оболочку через Capacitor

## Взаимодействие backend ↔ frontend

- Прод-стек: **nginx — единственная открытая точка** (`:80`): раздаёт SPA (`frontend/dist`), статику `/media/` и проксирует `/api`, `/healthz`, `/ready` на `gym-app:8080` (порт app наружу не публикуется)
- API: REST, JSON, базовый префикс `/api/v1`
- Формат ответов: envelope `{"status":"success","data":...}` / `{"status":"error","message":"..."}` (см. backend/AGENT.md)
- Локализация: фронт шлёт `Accept-Language` (`ru`/`en`); строки UI — во фронте (`frontend/src/locales/`), строки сущностей и ошибок — на бэкенде (отдаются локализованными)
- Локальная разработка: dev-сервер фронта `:5173` проксирует `/api` и `/media` → `localhost:8080` (CORS не нужен); в dev `/media/` отдаёт Gin из `backend/media`

## Команды (из корня репо)

```bash
make up            # front-build + docker compose up -d --build (mysql, redis, migrate, app, worker, nginx; GYM_ENV=prod внутри)
make start / stop  # запуск/остановка уже созданных контейнеров (первый раз — make up)
make ui            # + asynqmon (веб-UI очередей, :8081, профиль "ui")
make down / logs / ps
make run / worker  # go локально (GYM_ENV=dev; нужны mysql+redis: docker compose up -d mysql redis)
make build / test / vet / tidy
make migrate-up / migrate-down / migrate-status / migrate-create name=имя
make test-db-up    # тест-инфраструктура: mysql-test :3307 (gym_test), redis-test :6380 (профиль "test")
make test
make dataset-fetch  # клон датасета упражнений в data/exercises-dataset (GIF/миниатюры, ~136 МБ)
make seed-catalog  # импорт каталога упражнений в MySQL (1324 шт., ru/en, пути медиа; импортёр backend/cmd/importer)
make media-sync    # копирование GIF/миниатюр (~136 МБ) из data/exercises-dataset в backend/media
make front-install / front-dev / front-build / front-lint / front-format
```

Порты: nginx **:80** (SPA + /api + /media — единственная открытая точка стека), frontend dev :5173, app :8080 (только локальный dev через make run), asynqmon :8081, mysql :3306 (gym/gym), redis :6379, mysql-test :3307 (gym_test/gym_test), redis-test :6380.

## Правила

- **Git**: никаких операций (commit / push / amend и т.п.) без явного разрешения пользователя
- Комментарии в коде — только по просьбе
- Изменения внутри `backend/` и `frontend/` — следуя паттернам из их AGENT.md
