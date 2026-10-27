.PHONY: run worker test lint build

# Don't depend on the caller's env: turbo filters it and a global GO111MODULE=off breaks module builds.
export GO111MODULE=on

# Local secrets (DATABASE_URL, ...) live in services/api/.env, never committed.
-include .env
export

run:
	PORT=$${PORT:-8090} go run ./cmd/api

worker:
	go run ./cmd/worker

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "run gofmt -w ." && exit 1)

build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -o bin/worker ./cmd/worker
