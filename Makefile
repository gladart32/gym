BACKEND := backend
FRONTEND := frontend

# Default environment for local go-commands; can be overridden: GYM_ENV=prod make run
export GYM_ENV ?= dev

.PHONY: up down start stop logs ps ui test-db-up test-db-down \
        build run worker test vet tidy \
        migrate-up migrate-down migrate-status migrate-create \
        front-install front-dev front-build front-lint front-format

# --- docker (docker-compose.yml в корне) ---

up:
	docker compose up -d --build

down:
	docker compose down

start:
	docker compose start

stop:
	docker compose stop

logs:
	docker compose logs -f app worker

ps:
	docker compose ps

ui:
	docker compose --profile ui up -d asynqmon

test-db-up:
	docker compose --profile test up -d mysql-test redis-test

test-db-down:
	docker compose --profile test down

# --- backend (Go) ---

build:
	cd $(BACKEND) && go build -o bin/gym ./cmd/gym

run:
	cd $(BACKEND) && go run ./cmd/gym serve

worker:
	cd $(BACKEND) && GYM_LOGGER_FILE_PATH=logs/worker.log go run ./cmd/gym worker

test:
	cd $(BACKEND) && GYM_ENV=test go test ./...

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
