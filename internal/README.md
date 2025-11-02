# Internal layout (hexagonal)

- `domain/` – core entities and value objects used internally.
- `ports/` – interfaces (inbound/outbound) the core relies on.
- `adapters/` – implementations of ports for HTTP, storage, queues, etc.
  - `adapters/http` – HTTP server that wires generated OpenAPI handlers to ports.
  - `adapters/queue` – stub in-memory scanner queue (dev only).
  - `adapters/profiles` – stub profiles provider.
  - `adapters/companies` – stub company identity provider.
- `api/` – generated code from `api/openapi.yaml` (placed at `internal/api/`).
- `config/` – app configuration loading (env-first).

Generate the API package:

```
make generate
```

