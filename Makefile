BINARY := bin/gym

# Default environment for local commands; can be overridden: GYM_ENV=prod make ...
export GYM_ENV ?= dev

.PHONY: all build run worker tidy vet test test-db-up test-db-down up down logs ps ui migrate-up migrate-down migrate-status migrate-create

all: build

build:
	go build -o $(BINARY) ./cmd/gym

run:
	go run ./cmd/gym serve

worker:
	GYM_LOGGER_FILE_PATH=logs/worker.log go run ./cmd/gym worker

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	GYM_ENV=test go test ./...

test-db-up:
	docker compose --profile test up -d mysql-test redis-test

test-db-down:
	docker compose --profile test down

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f app worker

ps:
	docker compose ps

ui:
	docker compose --profile ui up -d asynqmon

migrate-up:
	go run ./cmd/gym migrate up

migrate-down:
	go run ./cmd/gym migrate down

migrate-status:
	go run ./cmd/gym migrate status

migrate-create:
	go run ./cmd/gym migrate create $(name)
