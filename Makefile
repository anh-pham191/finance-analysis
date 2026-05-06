.PHONY: db-up db-down migrate test test-integration lint sqlc help

export DATABASE_URL_OWNER ?= postgres://finance_owner:finance_owner_local_dev_only@localhost:15432/finance?sslmode=disable
export DATABASE_URL_APP ?= postgres://finance_app:finance_app_local_dev_only@localhost:15432/finance?sslmode=disable

db-up:
	docker compose up -d

db-down:
	docker compose down

migrate:
	go run ./cmd/cli migrate up

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	golangci-lint run ./...

sqlc:
	sqlc generate -f internal/storage/postgres/sqlc.yaml

help:
	@echo "targets: db-up db-down migrate test test-integration lint sqlc"
