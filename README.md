# Camille — Digital Nutrition Label for Websites

Camille is an OpenAPI‑first Go service that analyzes websites to produce an explainable, evidence‑backed “digital nutrition label” for users. It focuses on privacy, security, and governance signals and is designed with a hexagonal (ports/adapters) architecture, PostgreSQL storage, and a background job runner.

This repo currently ships a working API skeleton with PostgreSQL, a job runner, and a clean path to add an AI‑assisted policy processor (see tasks/processor.md).

## Features (MVP)
- OpenAPI‑driven HTTP API (chi) with generated handlers
- PostgreSQL schema and migrations (goose)
- Hexagonal layout with clear ports, services, and adapters
- Scan queue + background workers (FOR UPDATE SKIP LOCKED)
- Blocking scan option for testing: `POST /scan?wait=true`
- Profiles endpoint reading persisted scores
- Task plan to add an AI policy parser and scoring (tasks/processor.md)

## Architecture
- Language: Go 1.25
- Web: chi router, handlers generated via oapi‑codegen (strict server)
- Storage: PostgreSQL 16 (docker‑compose), `pgx` pool
- Jobs: DB‑backed queue with `FOR UPDATE SKIP LOCKED` workers
- Layout (hexagonal):
  - `api/openapi.yaml` — API spec
  - `internal/api/` — generated types + server (do not edit)
  - `internal/ports/` — interfaces (inbound/outbound)
  - `internal/services/` — use‑case logic (scanner, profiles, companies)
  - `internal/adapters/http/` — HTTP server wiring to ports
  - `internal/adapters/postgres/` — pgx connection + repositories + jobs
  - `internal/workers/scanrunner/` — worker loop + processor interface
  - `db/migrations/` — goose SQL migrations
  - `cmd/server/` — API + optional in‑process workers

## Quick Start
Prerequisites
- Go 1.25+
- Docker + docker compose
- Make

Setup
- Copy env file and adjust as needed:
  - `cp .env.example .env`
- Start PostgreSQL and apply migrations:
  - `make db/up`
  - `make db/migrate`
- Generate API code (from OpenAPI):
  - `make generate`
- Run the server (loads `.env` automatically):
  - `make run`

Environment variables (common)
- `APP_ENV` — `development` (default)
- `LISTEN_ADDR` — HTTP listen address, e.g. `:8080`
- `DATABASE_URL` — e.g. `postgres://camille:camille@localhost:5432/camille?sslmode=disable`
- `SCAN_WORKERS` — number of background scan workers (0 disables workers). You can still use blocking scans for testing.

## API (essentials)
OpenAPI spec: `api/openapi.yaml`

Endpoints
- `GET /healthz` — liveness probe
- `POST /scan` — enqueue a scan, returns 202 `{scan_id}`
  - Query params: `wait` (bool), `timeout` (seconds). If `wait=true`, blocks and returns 200 with final `ScanResponse`.
- `GET /scans/{id}` — check scan status and progress
- `GET /profiles/{domain}` — fetch latest profile (scores, badges, issues when available)
- `GET /companies/{opencorporates_id}` — identity snapshot (stub)

Examples
- Health: `curl -s localhost:8080/healthz`
- Enqueue (non‑blocking):
  - `curl -s -X POST localhost:8080/scan -H 'content-type: application/json' -d '{"url":"https://example.com"}'`
- Enqueue (blocking for testing):
  - `curl -s -X POST 'localhost:8080/scan?wait=true&timeout=30' -H 'content-type: application/json' -d '{"url":"https://example.com"}'`
- Status: `curl -s localhost:8080/scans/<scan_id>`
- Profile: `curl -s localhost:8080/profiles/example.com`

## Development Guide
Codegen
- Edit `api/openapi.yaml` → `make generate` to regenerate `internal/api/api.gen.go`.

Database
- Start DB: `make db/up`
- Migrate: `make db/migrate`
- Status: `make db/status`
- Down (remove volumes): `make db/down`

Workers and Blocking Scans
- Background workers start when `SCAN_WORKERS > 0` (in the same process as the API).
- Blocking scans (`wait=true`) run the same processor synchronously for that scan. Useful for dev/tests.

Structure (selected files)
- Server entry: `cmd/server/main.go`
- HTTP adapter: `internal/adapters/http/server.go`
- Postgres adapter: `internal/adapters/postgres/db.go`
- Jobs (queue): `internal/adapters/postgres/jobs.go`
- Ports: `internal/ports/*.go`
- Services: `internal/services/*`
- Workers: `internal/workers/scanrunner/runner.go`

## Processor Roadmap (AI Policy Parser)
The worker pipeline currently includes a `ScanProcessor` interface and a Noop processor for scaffolding. The next step is to implement an AI‑assisted policy processor that extracts evidence and computes a privacy score.

- MVP plan and acceptance checklist: `tasks/processor.md`
- Key additions (MVP):
  - New tables: `evidence`, `signals` (non‑breaking migration)
  - `AIExtractor` adapter (OpenAI/Ollama)
  - PipelineProcessor to fetch → chunk → AI extract → normalize → score
  - Deterministic scoring writes to existing `scores` table (privacy only initially)

## Design Notes
- OpenAPI‑first: consistent contracts and generated handlers
- Hexagonal architecture: ports define capabilities; adapters implement them
- Deterministic scoring: transparent, evidence‑backed inputs
- Idempotent scans: domain normalization (eTLD+1) and queued jobs
- Postgres concurrency: `FOR UPDATE SKIP LOCKED` for safe multi‑worker claims

## Troubleshooting
- Missing `go.sum` entries after codegen → run `go mod tidy`
- Cannot connect to DB → verify `DATABASE_URL`, `make db/up`, and migrations
- Port in use → change `LISTEN_ADDR` or stop conflicting process
- 404 on `/profiles/{domain}` → no score stored yet; run a blocking scan or seed a score row

## Acknowledgements (planned integrations)
- ToS;DR, OpenCorporates, OpenSanctions, Mozilla Observatory, DuckDuckGo Tracker Radar, WikiRate, HSTS preload list, security.txt

## License
TBD
