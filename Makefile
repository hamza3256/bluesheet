.PHONY: build test docker-up docker-down migrate run-api run-worker

build:
	go build -o bin/bluesheet ./cmd/bluesheet

test:
	go test -v -count=1 ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down -v

migrate:
	go run ./cmd/bluesheet migrate

run-api:
	go run ./cmd/bluesheet api

run-worker:
	go run ./cmd/bluesheet worker
