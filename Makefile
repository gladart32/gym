BINARY := bin/gym

.PHONY: all build run worker tidy vet up down logs ps ui migrate-up migrate-down migrate-status migrate-create

all: build

build:
	go build -o $(BINARY) ./cmd/gym

run:
	go run ./cmd/gym serve

worker:
	go run ./cmd/gym worker

tidy:
	go mod tidy

vet:
	go vet ./...

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
