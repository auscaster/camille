# Digital Nutrition Label – Full Scope v1.0

A complete scope for an MVP that rates a website’s privacy, security, and governance posture the moment a user visits, inspired by “Yuka for the web.” Deliverables include a **desktop browser extension** and a **mobile-friendly PWA** backed by a **Go API**.

---

## 0) Executive Summary

**Goal:** Give users an instant, explainable “nutrition label” for any website based on verifiable signals (policy text, headers, security checks, corporate identity, ownership, sanctions, basic ESG). Every signal links to **evidence**. No black boxes.

**Clients:**

* **Browser Extension (MV3)**: Best UX on desktop, can surface on-page signals.
* **Web App / PWA**: Paste/share a URL to see the same label on mobile or desktop; supports watchlists and alerts.

**Backend:** Go (chi + oapi-codegen), Postgres, object storage for artifacts, async jobs for scanners, deterministic scoring.

**Principles:** Evidence-first, transparent weights, conservative claims, privacy-by-design.

---

## 1) Objectives & KPIs

**Objectives**

* Provide an **instant, understandable label** (≤800ms cached, ≤4s cold) for any domain.
* **Explainability:** every signal shows source evidence (quote, header, API response link).
* **Coverage:** support top 10k domains quickly; degrade gracefully on long tail.
* **Trust:** zero dark-patterns; optional account; user can delete data.

**KPIs**

* TTI for cached profile: **≤800ms P95**.
* New scan P95: **≤4s** for first visible partials, **≤30s** to final.
* Evidence completeness: **≥90%** of surfaced signals link to a source artifact.
* False positive rate for critical claims (e.g., “sells data”): **<2%** on gold set.
* Weekly active watchlist users / retention; alert CTR.

---

## 2) Target Users & Jobs To Be Done

**Personas**

1. **Everyday User**: “Is this site safe with my data?”
2. **Power User/Privacy Advocate**: “Why did you grade it that way? Show me receipts.”
3. **Procurement/IT Reviewer**: “Quick risk snapshot before approving a vendor.”

**JTBD**

* Decide whether to trust a site at a glance (grades + badges).
* Learn the **top 3 risks** in plain English, with links to the proof.
* Watch domains and get notified when policies/security posture change.

---

## 3) Product Scope (MVP)

### 3.1 Core Features

* **Domain Profile**: Overall grade, sub-scores (Privacy/Security/Governance/Social-Env), badges.
* **Evidence Drawer**: Per-signal source: quoted policy span, HTTP header snapshot, API JSON, etc.
* **On-Demand Scanning**: Triggered when a user visits/shares a URL; idempotent.
* **Watchlist & Alerts**: Track domains; get push/email when signals change.
* **Public Share Link**: Read-only profile page for a domain.

### 3.2 Client Features

**Extension (desktop)**

* Auto-detect origin on navigation; show label popover.
* Optional local telemetry: count third-party requests; map to tracker entities.
* One-click “Watch this site.”

**PWA (mobile/desktop)**

* Paste/share URL; show same label.
* Web Share Target (Android), iOS Shortcut & bookmarklet.
* Push notifications for changes (install to Home Screen for iOS).

### 3.3 Out of Scope (MVP)

* Paid ESG feeds (MSCI/Sustainalytics), deep controversy mining.
* Full breach-history curation; complex jurisdictional compliance checklists.

---

## 4) Non‑Functional Requirements

* **Performance**: See KPIs above; streaming partials.
* **Reliability**: 99.9% monthly API availability. At-least-once job execution with idempotency keys.
* **Security**: TLS 1.2+, HSTS; secrets via cloud KMS; least-privileged.
* **Privacy**: Clients send **origin only** (no path/query); minimize telemetry; user deletion.
* **Accessibility**: WCAG 2.2 AA; keyboard-only operation; reduced motion mode.
* **Internationalization**: English first; strings externalized; locale-friendly numbers/dates.

---

## 5) Architecture Overview

**Components**

* **API (Go/chi)**: REST endpoints; oapi-codegen; Problem+JSON errors.
* **Job Runner**: Async scans (HTTP fetch, headless browse, header/security checks, registries).
* **Adapters**: Policy fetch/render, security scanners, registry lookups, sanctions, ESG.
* **Scoring Engine**: Deterministic function over normalized `signals`.
* **Storage**: Postgres (normalized), Object Storage (artifacts), Cache (Redis), Queue (Redis/Asynq or equivalent).

**Data Flow**

1. Client calls `POST /scan {url}` or `GET /profiles/:domain?refresh_if_stale=1`.
2. API normalizes origin → enqueues scan if stale/missing → returns cached/partial profile.
3. Workers fetch policies, render to text, run rule-based extractors and targeted LLM classifiers.
4. Workers run scanners (TLS/headers, HSTS/security.txt, DMARC/SPF/DKIM, tracker census, registry lookups, sanctions, ESG).
5. Normalize to `signals[]` with evidence references; compute `scores` and badges; persist.
6. Clients poll/subscribe for updated profile; UI updates progressively.

---

## 6) Integrations (MVP-friendly)

* **Terms & Privacy crowd-signal**: ToS;DR (read-only).
* **Company Registry**: OpenCorporates (public endpoints).
* **Beneficial Ownership**: OpenOwnership/BODS where available.
* **Sanctions/PEPs**: OpenSanctions (API).
* **Security Checks**: Mozilla Observatory, SSL Labs (API), plus in-house header checks.
* **Tracker Mapping**: DuckDuckGo Tracker Radar (dataset).
* **Email Auth**: DNS lookups for SPF/DKIM/DMARC.

> All adapters designed with timeouts, retries, caching, and explicit `retrieved_at` timestamps. Missing data is marked **Unknown**, not penalized.

---

## 7) Scoring Model

**Weights (v1.0)**

* Privacy **40%**
* Security **20%**
* Governance **25%**
* Social/Environment **15%**

**Hard Rules**

* Positive sanctions hit → cap overall to **D** (or show “High Risk”).
* Critical header failures (no HTTPS) → hard deduction.

**Unknowns**

* Unknown data yields neutral weight (neither penalize nor award). Surface “Unknown” explicitly.

**Versioning**

* Include `method_version` on scores; store rubric as a signed JSON document.

---

## 8) Signal Catalogue (with detection method & evidence)

### 8.1 Privacy Signals

* **Data sale/sharing**: regex + keyword catalog (sell/share, advertising partners), confirm with policy quote. Evidence: quoted span + policy URL.
* **Profiling/targeted ads**: regex + LLM confirm on relevant sections. Evidence: span.
* **Model training**: look for “train”, “improve our models”; LLM confirm. Evidence: span.
* **Retention clarity**: presence of specific retention windows. Evidence: span.
* **User rights & DSR**: presence of access/deletion/portability; working contact/email/form. Evidence: span + link.
* **Cross-border transfers**: detect statements about transfers; legal basis, SCCs. Evidence: span.
* **Children/sensitive data**: detect minors/biometrics/health/finance clauses. Evidence: span.
* **Dark patterns**: language such as “call/fax/mail to cancel”, pre-ticked boxes; heuristic. Evidence: span.
* **GPC handling**: server-side test with GPC header + observe CMP behavior (best effort). Evidence: server scan log.

### 8.2 Security Signals

* **HTTPS + HSTS**: header check; preload list status. Evidence: response headers + preload status.
* **CSP / Referrer-Policy / Permissions-Policy / XFO**: presence and quality heuristics. Evidence: headers snapshot.
* **TLS quality**: SSL Labs grade API; expiry window. Evidence: API JSON excerpt.
* **security.txt**: presence at `/.well-known/security.txt`. Evidence: fetched file hash + snippet.
* **Email auth**: SPF, DKIM, DMARC existence and DMARC policy. Evidence: DNS TXT records.

### 8.3 Governance Signals

* **Entity resolution**: domain ↔ legal entity match in OpenCorporates; confidence score. Evidence: registry record link.
* **Beneficial ownership transparency**: ownership data presence in BODS-supplying registries. Evidence: BODS/registry record link.
* **Sanctions/PEP screening**: of company/officers. Evidence: OpenSanctions match JSON.
* **Domain integrity** (phase 2): CT logs/W­HOIS changes; domain age. Evidence: CT/WHOIS snapshot.

### 8.4 Social/Environment Signals

* **Policy presence**: human-rights policy, modern-slavery statement, climate policy, whistleblower policy. Evidence: URLs.
* **Open ESG metrics**: selected indicators from open sources (e.g., WikiRate). Evidence: API data with source.

### 8.5 Tracker Intensity (desktop-enhanced)

* **Third-party request density**: % of third-party requests on first paint.
* **Tracker ownership concentration**: count unique tracker owners.
* Evidence: extension telemetry or headless server capture; Tracker Radar mapping.

---

## 9) Data Model (Postgres)

**Entities**

* `domains(id, registrable_domain, first_seen_at, last_scan_at)`
* `companies(id, name, jurisdiction, registry_id, website, confidence)`
* `domain_company_map(domain_id, company_id, method, confidence, created_at)`
* `scans(id, domain_id, started_at, finished_at, status, error_code, method_version)`
* `artifacts(id, scan_id, type, url, sha256, bytes, created_at)`
* `signals(id, scan_id, code, value, severity, confidence, source, retrieved_at, details jsonb)`
* `evidence(id, signal_id, source_url, quote, hash, raw jsonb)`
* `scores(id, domain_id, privacy, security, governance, social_env, overall, method_version, computed_at, badges text[])`
* `watchlists(id, user_id, domain_id, created_at)`

**Indexes**

* `domains(registrable_domain unique)`
* `signals(scan_id, code)`; GIN on `signals.details`
* `scores(domain_id, computed_at desc)`

---

## 10) API Design (REST)

**Error Shape**: Problem+JSON

### Endpoints

* `POST /scan` → request a scan

```json
{ "url": "https://example.com" }
```

Response:

```json
{ "scan_id": "scn_01H...", "status": "queued" }
```

* `GET /scans/{id}` → scan status + partial results
* `GET /profiles/{domain}` → latest profile

```json
{
  "domain": "example.com",
  "overall": 78,
  "scores": {"privacy":72, "security":85, "governance":80, "social_env":60},
  "badges": ["HSTS Preloaded", "security.txt present"],
  "signals": [{"code":"policy-data-sale", "severity":"high", "summary":"Shares data with ad partners", "confidence":0.92}],
  "evidence": [{"signal":"policy-data-sale","source_url":"https://.../privacy","quote":"We share data with..."}],
  "computed_at": "2025-11-02T05:45:00Z",
  "method_version": "1.0.0"
}
```

* `POST /watch` → add domain to watchlist
* `DELETE /watch/{domain}` → remove
* `GET /watch` → list
* `GET /companies/{registry_id}` → company snapshot

**OpenAPI Snippet**

```yaml
openapi: 3.1.0
info:
  title: Digital Label API
  version: 1.0.0
paths:
  /scan:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [url]
              properties:
                url:
                  type: string
                  format: uri
      responses:
        "202":
          description: Accepted
          content:
            application/json:
              schema:
                type: object
                properties:
                  scan_id: { type: string }
                  status: { type: string, enum: [queued, started] }
```

---

## 11) Browser Extension Spec (MV3)

**Manifest**

* Permissions: `activeTab`, `storage`, `declarativeNetRequestWithHostAccess`, `notifications` (optional), host permissions for `*://*/*` (scoped by user action where possible).
* Background: service worker (fetch API, alarm for re-scan reminders).
* Content script: injects panel UI; requests origin; shows label.

**Flows**

1. On navigation to a new origin → debounce → call `/profiles/:domain?refresh_if_stale=1`.
2. Show cached results immediately; if stale, show spinner and update when ready.
3. On user click “Watch”, POST `/watch`.

**Storage**

* `chrome.storage.sync` for lightweight preferences; large data remains server-side.

**UI**

* Popover with Overall grade, top 3 risks, badges; link to “View Evidence”.

---

## 12) Web App / PWA Spec

**Routes**

* `/` home, paste/share URL
* `/profile/:domain` public profile
* `/watchlist` personal list

**Capabilities**

* Installable PWA, offline shell for last-viewed profiles
* Web Share Target (Android)
* iOS Shortcut + bookmarklet generator
* Web Push (Chrome/Android; iOS when installed to Home Screen)

**Performance**

* Static prerender shell; dynamic hydrate on data arrival; CDN caching for profiles with `ETag`/`Cache-Control`.

---

## 13) LLM Strategy & Quality

**Extraction** (targeted, span-first)

* Pre-scan with regex/keyword rules to isolate suspect sections (sale, profiling, AI, transfers, retention, children, DSR).
* LLM classifies **only** isolated chunks and must return `{label, span_quote, span_start, span_end}`; reject outputs without span.

**Summaries**

* Deterministic assembly of bullets from accepted facts; no freeform LLM prose in UI.

**Guardrails**

* Zero-trust parsing; JSON schema validation; hallucination drop rule.
* Gold test set (50+ sites) with expected signals; CI gates.

**Config**

* Temperature: 0; max tokens small; per-tenant model keys abstracted.

---

## 14) Privacy, Security, Legal

**Data Minimization**

* Collect origin (eTLD+1) only; never store paths/queries.
* Hash origins at rest for telemetry; separate from profiles.

**User Rights**

* Export/delete account and watchlist.

**Legal Language**

* Present “Risk Signals” and quotes; avoid categorical statements. Provide right-of-reply channel for corrections with evidence.

**Security**

* HTTPS/HSTS; CSP on web app; rotate keys; audit logs for profile edits.

**Retention**

* Artifacts: 90 days default; signals/scores: 1 year; user can purge.

---

## 15) Observability & Ops

* **Metrics**: scan duration, cache hit rate, adapter error rates, signal coverage, false positive rate (from adjudicated feedback).
* **Logging**: structured; correlation IDs per scan.
* **Tracing**: OpenTelemetry for API + workers.
* **Dashboards**: latency, throughput, errors by adapter.
* **Alerts**: adapter outage, queue backlog, error spikes.

---

## 16) QA & Testing Plan

* **Unit**: regex extractors, scoring function, adapters with fixtures.
* **Integration**: end-to-end scan with mock HTTP servers.
* **E2E**: extension UI flows (Playwright) and PWA routes.
* **Security**: dependency scans, header checks on our app, secret scans.
* **Load**: P95 cold scan under 30s at 50 RPS enqueue rate.
* **Cross-browser**: Chrome, Edge; Firefox later (policies differ).

---

## 17) Deployment & Environments

* **Envs**: dev, staging, prod; separate projects/DBs/buckets.
* **CI/CD**: lint, tests, build Docker images, migrate DB, canary deploy.
* **Infra**: container runtime (e.g., ECS/K8s), Redis (cache/queue), Postgres (managed), Object storage (S3/GCS), CDN for profiles.
* **Secrets**: managed KMS; no secrets in env files.

---

## 18) Roadmap & Milestones

**Milestone A – Core Scan & Label (2–3 weeks)**

* API `/scan`, `/profiles/:domain` with caching and partials.
* Policy fetch/render; privacy extractors (sale/sharing, profiling, retention, DSR, transfers, AI training).
* Security checks: HTTPS/HSTS, CSP, Referrer-Policy, security.txt, TLS grade (API), DMARC/SPF.
* Governance: OpenCorporates lookup; sanctions screen.
* Deterministic scoring v1.0 + badges.
* Extension popover + PWA profile view.
* **Acceptance**: label with top 3 risks + evidence works on top 500 sites.

**Milestone B – Watchlists & Alerts (1–2 weeks)**

* Watchlist CRUD, push/email notifications, diff engine for policy/headers.
* Web Share Target (Android), iOS Shortcut/bookmarklet.
* **Acceptance**: user gets alert within 24h of profile change; share-flow works.

**Milestone C – Tracker Census & ESG (2 weeks)**

* Desktop tracker intensity; ownership concentration; basic open ESG metrics/policy presence.
* Public profile pages with OG image cards.
* **Acceptance**: tracker signals visible on desktop; ESG section populated where available.

---

## 19) Monetization & Packaging

* **Free**: label, badges, evidence.
* **Pro**: watchlists >10 domains, alerts frequency, CSV export, historical deltas.
* **B2B API**: bulk domain scoring, SLA, export.

---

## 20) Risks & Mitigations

* **Patchy ESG data**: mark Unknown; design adapters for future paid feeds.
* **LLM drift**: regression tests; prompt versioning; small, targeted contexts.
* **Rate limits**: aggressive caching; user-backoff; queue smoothing.
* **Legal pushback**: evidence-first language; right-of-reply workflow; moderation log.

---

## 21) Open Questions

* Should tracker telemetry be **extension-only** or also server-side headless?
* Minimum evidence set for a signal to appear (one quote vs. multiple corroborations)?
* Badge thresholds (e.g., what CSP quality qualifies?).
* Public profiles: opt-out policy for domains?

---

## 22) Appendices

### 22.1 JSON Schemas

**Signal**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Signal",
  "type": "object",
  "required": ["code", "value", "confidence", "source", "retrieved_at"],
  "properties": {
    "code": {"type": "string"},
    "value": {},
    "severity": {"type": "string", "enum": ["info","low","medium","high"]},
    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "source": {"type": "string"},
    "retrieved_at": {"type": "string", "format": "date-time"},
    "details": {"type": "object"}
  }
}
```

**Evidence**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Evidence",
  "type": "object",
  "required": ["signal","source_url","hash"],
  "properties": {
    "signal": {"type": "string"},
    "source_url": {"type": "string", "format": "uri"},
    "quote": {"type": "string"},
    "hash": {"type": "string"},
    "raw": {}
  }
}
```

### 22.2 Postgres DDL (excerpt)

```sql
create table domains (
  id bigserial primary key,
  registrable_domain text not null unique,
  first_seen_at timestamptz default now(),
  last_scan_at timestamptz
);

create table scans (
  id bigserial primary key,
  domain_id bigint not null references domains(id) on delete cascade,
  started_at timestamptz default now(),
  finished_at timestamptz,
  status text check (status in ('queued','running','done','error')),
  error_code text,
  method_version text
);

create table artifacts (
  id bigserial primary key,
  scan_id bigint not null references scans(id) on delete cascade,
  type text not null,
  url text,
  sha256 text,
  bytes bigint,
  created_at timestamptz default now()
);

create table signals (
  id bigserial primary key,
  scan_id bigint not null references scans(id) on delete cascade,
  code text not null,
  value jsonb,
  severity text,
  confidence double precision,
  source text,
  retrieved_at timestamptz,
  details jsonb,
  unique (scan_id, code)
);

create table evidence (
  id bigserial primary key,
  signal_id bigint not null references signals(id) on delete cascade,
  source_url text,
  quote text,
  hash text,
  raw jsonb
);

create table scores (
  id bigserial primary key,
  domain_id bigint not null references domains(id) on delete cascade,
  privacy int,
  security int,
  governance int,
  social_env int,
  overall int,
  badges text[],
  method_version text,
  computed_at timestamptz default now()
);
```

### 22.3 Scoring Config (YAML example)

```yaml
version: 1.0.0
weights:
  privacy: 0.40
  security: 0.20
  governance: 0.25
  social_env: 0.15
rules:
  hard_caps:
    sanctions_hit: D
  privacy:
    data_sale: { yes: -25, unclear: -10, no: 0 }
    gpc_honored: { yes: +10, no: -10, unknown: 0 }
    retention_specific: { yes: +6, no: -6 }
  security:
    https: { yes: +10, no: -40 }
    hsts_preload: { yes: +5 }
    tls_grade: { A+: +10, A: +8, B: +4, C: 0, D: -6, F: -10 }
  governance:
    entity_match_confidence: { high: +8, medium: +4, low: 0 }
    ownership_transparency: { yes: +6, partial: +3, no: 0 }
  social_env:
    hr_policy_present: { yes: +3 }
    modern_slavery_statement: { yes: +4 }
```

### 22.4 Analytics Events (client)

* `profile_viewed { domain, from: extension|pwa }`
* `watch_added { domain }`
* `alert_clicked { domain, signal_code }`
* `evidence_opened { domain, signal_code }`

### 22.5 Glossary

* **Signal**: a normalized fact with a value, confidence, and source.
* **Evidence**: the raw proof supporting a signal.
* **Badge**: a memorable, single-dimension achievement derived from signals (e.g., “Honors GPC”).
* **Profile**: the public aggregation (scores + signals + evidence) for a domain.