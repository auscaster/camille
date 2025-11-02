-- +goose Up
-- baseline schema
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for gen_random_uuid

CREATE TABLE IF NOT EXISTS domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registrable_domain TEXT NOT NULL UNIQUE,
    company_ref UUID NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE scan_status AS ENUM ('queued','running','completed','failed');

CREATE TABLE IF NOT EXISTS scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    status scan_status NOT NULL DEFAULT 'queued',
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    progress REAL NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS scan_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id UUID NOT NULL UNIQUE REFERENCES scans(id) ON DELETE CASCADE,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    status scan_status NOT NULL DEFAULT 'queued',
    attempts INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_name TEXT NOT NULL,
    jurisdiction TEXT NULL,
    registry_id TEXT NULL,
    website TEXT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS scores (
    domain_id UUID PRIMARY KEY REFERENCES domains(id) ON DELETE CASCADE,
    privacy INT NOT NULL,
    security INT NOT NULL,
    governance INT NOT NULL,
    esg INT NOT NULL,
    overall INT NOT NULL,
    badges JSONB NOT NULL DEFAULT '[]'::jsonb,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    method_version TEXT NOT NULL DEFAULT 'v0'
);

CREATE INDEX IF NOT EXISTS idx_scans_domain ON scans(domain_id);

-- +goose Down
DROP TABLE IF EXISTS scores;
DROP TABLE IF EXISTS companies;
DROP TABLE IF EXISTS scan_jobs;
DROP TABLE IF EXISTS scans;
DROP TYPE IF EXISTS scan_status;
DROP TABLE IF EXISTS domains;

