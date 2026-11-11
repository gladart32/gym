BACKEND := backend
FRONTEND := frontend
DATA_DIR ?= data
DATASET_DIR ?= $(DATA_DIR)/exercises-dataset

COMPOSE ?= docker compose
# с профилями: stop/down/start/ps видят все контейнеры проекта,
# включая тест-БД (profile "test") и asynqmon (profile "ui")
COMPOSE_ALL ?= $(COMPOSE) --profile test --profile ui

-include .env

# Default environment for local go-commands; can be overridden: GYM_ENV=prod make run
export GYM_ENV ?= dev

MYSQL_USER ?= gym
MYSQL_PASSWORD ?= gym
MYSQL_DATABASE ?= gym
MYSQL_PORT ?= 3306

.PHONY: up down start stop logs ps ui test-db-up test-db-down \
        dataset-fetch seed-catalog media-sync \
        build run worker test vet tidy \
        migrate-up migrate-down migrate-status migrate-create \
        front-install front-dev front-build front-lint front-format

# --- docker (docker-compose.yml в корне) ---

up: front-build
	$(COMPOSE) up -d --build

down:
	$(COMPOSE_ALL) down

start:
	$(COMPOSE_ALL) start

stop:
	$(COMPOSE_ALL) stop

logs:
	$(COMPOSE) logs -f app worker

ps:
	$(COMPOSE_ALL) ps

ui:
	$(COMPOSE) --profile ui up -d asynqmon

test-db-up:
	$(COMPOSE) --profile test up -d mysql-test redis-test

# останавливает только тест-контейнеры (тома gym_test сохраняются)
test-db-down:
	$(COMPOSE) --profile test stop mysql-test redis-test

# --- backend (Go) ---

build:
	cd $(BACKEND) && go build -o bin/gym ./cmd/gym

run:
	cd $(BACKEND) && go run ./cmd/gym serve

worker:
	cd $(BACKEND) && GYM_LOGGER_FILE_PATH=logs/worker.log go run ./cmd/gym worker

# -p 1: интеграционные тесты (auth/exercises/programs) сеют одну тестовую БД —
# пакеты должны выполняться последовательно.
test:
	cd $(BACKEND) && GYM_ENV=test go test -p 1 ./...

vet:
	cd $(BACKEND) && go vet ./...

tidy:
	cd $(BACKEND) && go mod tidy

migrate-up:
	cd $(BACKEND) && go run ./cmd/gym migrate up

migrate-down:
	cd $(BACKEND) && go run ./cmd/gym migrate down

migrate-status:
	cd $(BACKEND) && go run ./cmd/gym migrate status

migrate-create:
	cd $(BACKEND) && go run ./cmd/gym migrate create $(name)

# --- данные каталога (переводы в data/ — в git; датасет клонируется, в git не попадает) ---

# Клон датасета упражнений (GIF и миниатюры, ~136 МБ) в data/exercises-dataset.
dataset-fetch:
	@test -d $(DATASET_DIR)/data || git clone --depth 1 https://github.com/hasaneyldrm/exercises-dataset $(DATASET_DIR)

# Импорт каталога упражнений в MySQL Gym (1324 упражнения, ru/en переводы, пути медиа).
# Флаги импортёра: make seed-catalog SEED_ARGS="-dry-run"  (или -update, -check-media=false)
seed-catalog:
	@test -f $(DATASET_DIR)/data/exercises.json || { echo "датасет не найден: $(DATASET_DIR) (сначала make dataset-fetch)"; exit 1; }
	cd $(BACKEND) && go run ./cmd/importer \
		-dsn '$(MYSQL_USER):$(MYSQL_PASSWORD)@tcp(127.0.0.1:$(MYSQL_PORT))/$(MYSQL_DATABASE)?charset=utf8mb4&parseTime=true&loc=UTC' \
		-dataset ../$(DATASET_DIR) \
		$(SEED_ARGS)

# Копирование GIF и миниатюр (~136 МБ) из data/exercises-dataset в backend/media —
# их раздаёт nginx (prod) и Gin (dev).
media-sync:
	@test -d $(DATASET_DIR)/videos || { echo "датасет не найден: $(DATASET_DIR) (сначала make dataset-fetch)"; exit 1; }
	mkdir -p $(BACKEND)/media
	cp -R $(DATASET_DIR)/videos $(BACKEND)/media/
	cp -R $(DATASET_DIR)/images $(BACKEND)/media/

# --- frontend (Vue) ---

front-install:
	cd $(FRONTEND) && npm install

front-dev:
	cd $(FRONTEND) && npm run dev

front-build:
	cd $(FRONTEND) && npm run build

front-lint:
	cd $(FRONTEND) && npm run lint

front-format:
	cd $(FRONTEND) && npm run format
