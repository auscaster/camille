SHELL := /bin/bash

OPENAPI := api/openapi.yaml
API_OUT := internal/api/api.gen.go
OAPI := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen

.PHONY: all generate run tidy fmt clean db/create db/migrate db/status

all: generate

generate: $(OPENAPI)
	@mkdir -p internal/api
	@echo "[oapi-codegen] Generating $(API_OUT) from $(OPENAPI)"
	@go run $(OAPI) -package api -generate types,chi-server,strict-server -o $(API_OUT) $(OPENAPI)

run:
	@bash -lc ' \
		if [ -f .env ]; then set -o allexport; source .env; set +o allexport; fi; \
		go run ./cmd/server \
	'

tidy:
	@go mod tidy

fmt:
	@go fmt ./...

clean:
	@rm -f $(API_OUT)

DATABASE_URL ?= postgresql://postgres:password@localhost:5432/camille?sslmode=disable
GOOSE := github.com/pressly/goose/v3/cmd/goose

db/create:
	@bash -lc ' \
		if [ -f .env ]; then set -o allexport; source .env; set +o allexport; fi; \
		if [ -z "$$DATABASE_URL" ]; then echo "DATABASE_URL is not set" >&2; exit 1; fi; \
		ADMIN_URL=$$(echo "$$DATABASE_URL" | sed -E "s@^((postgres(ql)?://[^/]+))/[^?]*@\1/postgres@"); \
		DB_NAME=$$(echo "$$DATABASE_URL" | sed -E "s@^[^/]+//[^/]+/([^?]+).*@\1@"); \
		echo "[db] ensuring database '$$DB_NAME' exists"; \
		EXISTS=$$(psql "$$ADMIN_URL" -Atqc "SELECT 1 FROM pg_database WHERE datname='$$DB_NAME'"); \
		if [ "$$EXISTS" = "1" ]; then \
			echo "[db] database '$$DB_NAME' already exists"; \
		else \
			echo "[db] creating database '$$DB_NAME'"; \
			psql "$$ADMIN_URL" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$$DB_NAME\""; \
		fi \
	'

db/migrate:
	@bash -lc ' \
		if [ -f .env ]; then set -o allexport; source .env; set +o allexport; fi; \
		echo "[goose] migrating on $$DATABASE_URL"; \
		go run $(GOOSE) -dir db/migrations postgres "$$DATABASE_URL" up \
	'

db/status:
	@bash -lc ' \
		if [ -f .env ]; then set -o allexport; source .env; set +o allexport; fi; \
		go run $(GOOSE) -dir db/migrations postgres "$$DATABASE_URL" status \
	'
