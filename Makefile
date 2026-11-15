.PHONY: run build test lint fmt migrate

# JWT_SECRET and the other settings are read from the environment; see
# .env.example.

run:
	go run ./cmd/api

build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api

test:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

migrate:
	go run ./cmd/api -migrate-only
