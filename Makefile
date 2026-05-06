.PHONY: db-up db-down migrate test test-integration lint sqlc mcp-build mcp-install help

MCP_BIN := $(HOME)/bin/finance-mcp
MCP_LAUNCHER := $(HOME)/bin/finance-mcp-launch.sh
MCP_ENV := $(HOME)/.config/finance-mcp/env

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

mcp-build:
	go build -o bin/finance-mcp ./cmd/mcp
	cp bin/finance-mcp $(MCP_BIN)

mcp-install: mcp-build
	@mkdir -p $(dir $(MCP_ENV))
	@grep -E '^(DATABASE_URL_APP|DATABASE_URL|AKAHU_BASE_URL)=' .env > $(MCP_ENV)
	@chmod 600 $(MCP_ENV)
	@cp bin/finance-mcp-launch.sh $(MCP_LAUNCHER)
	@chmod +x $(MCP_LAUNCHER)
	@echo "Installed:"
	@echo "  $(MCP_BIN)"
	@echo "  $(MCP_LAUNCHER)"
	@echo "  $(MCP_ENV) (mode 600)"
	@echo "Restart Claude Desktop to pick up changes."

help:
	@echo "targets: db-up db-down migrate test test-integration lint sqlc mcp-build mcp-install"
