# AI Policy Processor — MVP Scope (Compatible With Current API/DB)

## Objective
Ship a minimal, solid processor that fetches a site’s privacy/terms, uses AI to extract ethical and data‑storage issues with quotes, normalizes to signals, and computes a simple score. Everything is evidence‑backed; scoring remains deterministic. This plan is constrained to the existing OpenAPI responses and the current `scores` table.

## Compatibility Constraints
- OpenAPI: no changes. Endpoints and response shapes remain the same.
- Profiles: keep `internal/services/profiles` contract; it reads from `scores` via `GetLatestByDomain` and returns `api.Profile`.
- Scores table (existing): use fields `privacy`, `security`, `governance`, `esg`, `overall`, `badges`, `computed_at`, `method_version`.
  - MVP sets `security`, `governance`, `esg` to 0; `overall = privacy`.
  - Set `method_version` (e.g., `mvp-v1`), let `computed_at` default to `now()`; `badges` can be empty array.
- Additive migrations only: introduce new tables for `evidence` and `signals` without altering existing ones.
- Job runner and `/scan` semantics unchanged (optional `wait/timeout` already supported).

## MVP Deliverables
- PipelineProcessor implementing `scanrunner.ScanProcessor` (replaces NoopProcessor), injected in API and workers.
- One AI provider adapter behind an `AIExtractor` port (OpenAI or local Ollama), JSON‑only outputs.
- Minimal repos + migration for `evidence` and `signals`; extend ScoreRepository with an upsert method.
- Simple privacy‑only scoring; profiles read persisted scores unchanged.

## Minimal Data Model & Repos
- Migration `db/migrations/0002_policy_ai.sql` creates:
  - `evidence(id uuid default gen_random_uuid() primary key, scan_id uuid not null references scans(id) on delete cascade, source_type text not null, source_url text null, retrieved_at timestamptz not null default now(), hash text not null, payload jsonb not null, meta jsonb not null default '{}'::jsonb)`
  - `signals(id uuid default gen_random_uuid() primary key, domain_id uuid not null references domains(id) on delete cascade, code text not null, value_json jsonb not null, confidence real not null default 0, source text not null, retrieved_at timestamptz not null default now())`
  - Indexes: `idx_signals_domain_code(domain_id, code)`
- Repos (ports + pg adapters):
  - EvidenceRepo: `AddEvidence(ctx, scanID, sourceType, sourceURL, hash, payload, meta) (id string, err error)`
  - SignalsRepo: `UpsertSignals(ctx, domainID string, sigs []Signal) error` (upsert by `(domain_id, code, source, retrieved_at::date)` or simply insert for MVP)
  - ScoreRepository: add `UpsertScore(ctx, domainID string, score Scores, badges []string, methodVersion string) error`

## Signals Included (MVP)
- `policy.data.sale.present`
- `policy.ai.training.userdata`
- `policy.data.storage.retention.specified`
- `policy.data.storage.retention.indefinite`
- `policy.user.rights.deletion.channel.email_present`
- `policy.children.restrictions.stated`

## Pipeline (MVP)
1) Fetch & Extract Text
   - Discover policy URLs (anchor text heuristics + fallbacks `/privacy`, `/terms`).
   - HTTP client with timeouts (~10s), 1MB max body, same eTLD+1 only; block private IPs/ports.
   - Extract readable text with headings; compute `doc_hash`.
2) Chunk & Route (lightweight)
   - Split by headings/size; route likely chunks via keyword filters; cap 8 chunks per doc.
3) AI Extraction
   - Prompt returns strict JSON array of facts with mandatory `quote` and `section_url`.
   - Validate JSON; drop non‑conforming; store raw output in `evidence` with `model`, `prompt_version`, `prompt_hash`.
4) Normalize
   - Map facts → the 6 MVP signals; dedupe by `(code, section_url, quote_hash)`; average confidence.
5) Score (simple, deterministic)
   - Compute `privacy` from the 6 signals; set `overall = privacy`; set other subscores to 0.
   - Upsert into `scores` with `method_version = 'mvp-v1'` and empty `badges`.
6) Progress & Completion
   - Update progress at each phase via `JobRepository.UpdateScanProgress`; soft‑fail AI/network errors and continue.

## Ports & Adapters (MVP)
- `internal/ports/ai.go`: `AIExtractor.ExtractFacts(ctx, doc PolicyDoc) ([]Fact, error)` and data types.
- `internal/adapters/ai/openai.go` (or `ollama.go`) with timeouts and JSON‑only behavior.
- Minimal fetcher/readability utilities (can live under `internal/adapters/fetch/`).

## AI Extractor Implementation Details

### Port Definition (`internal/ports/ai.go`)
Define the AI extractor interface and data types:

```go
// AIExtractor processes policy documents and extracts structured facts
type AIExtractor interface {
    ExtractFacts(ctx context.Context, doc PolicyDoc) ([]Fact, error)
}

// PolicyDoc represents a document or chunk to be analyzed
type PolicyDoc struct {
    Content    string // The text content to analyze
    SourceURL  string // URL of the policy page
    SectionURL string // Optional: specific section/anchor URL
    Language   string // Optional: e.g., "en", "es"
}

// Fact represents a single extracted claim from a policy document
type Fact struct {
    Quote      string  // REQUIRED: Exact quote from the policy
    SectionURL string  // REQUIRED: URL including section anchor if available
    Category   string  // e.g., "data_sale", "ai_training", "retention"
    Confidence float64 // 0.0 to 1.0
    Metadata   map[string]interface{} // Optional additional context
}
```

### Adapter Implementation (`internal/adapters/ai/`)

#### OpenAI Adapter (`openai.go`)
```go
package ai

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "camille/internal/ports"
)

type OpenAIExtractor struct {
    APIKey         string
    Model          string // e.g., "gpt-4o-mini", "gpt-4o"
    Timeout        time.Duration
    PromptVersion  string // e.g., "mvp-v1"
    HTTPClient     *http.Client
}

type openAIRequest struct {
    Model       string      `json:"model"`
    Messages    []message   `json:"messages"`
    Temperature float64     `json:"temperature"`
    MaxTokens   int         `json:"max_tokens,omitempty"`
    ResponseFormat *struct {
        Type string `json:"type"`
    } `json:"response_format,omitempty"` // {"type": "json_object"} for JSON mode
}

type message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type openAIResponse struct {
    Choices []struct {
        Message struct {
            Content string `json:"content"`
        } `json:"message"`
    } `json:"choices"`
    Error *struct {
        Message string `json:"message"`
        Type    string `json:"type"`
    } `json:"error,omitempty"`
}

func (e *OpenAIExtractor) ExtractFacts(ctx context.Context, doc ports.PolicyDoc) ([]ports.Fact, error) {
    prompt := e.buildPrompt(doc)

    reqBody := openAIRequest{
        Model:       e.Model,
        Temperature: 0.1, // Low temperature for more deterministic output
        Messages: []message{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: prompt},
        },
        ResponseFormat: &struct{ Type string `json:"type"`}{Type: "json_object"},
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonData))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+e.APIKey)

    client := e.HTTPClient
    if client == nil {
        client = &http.Client{Timeout: e.Timeout}
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("api request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, string(body))
    }

    var apiResp openAIResponse
    if err := json.Unmarshal(body, &apiResp); err != nil {
        return nil, fmt.Errorf("unmarshal response: %w", err)
    }

    if apiResp.Error != nil {
        return nil, fmt.Errorf("openai error: %s", apiResp.Error.Message)
    }

    if len(apiResp.Choices) == 0 {
        return nil, fmt.Errorf("no choices in response")
    }

    return e.parseFactsJSON(apiResp.Choices[0].Message.Content, doc.SectionURL)
}

func (e *OpenAIExtractor) buildPrompt(doc ports.PolicyDoc) string {
    return fmt.Sprintf(`Analyze the following privacy policy text and extract key facts about data practices.

Document URL: %s

Policy Text:
%s

Extract facts in the following categories:
- data_sale: Information about selling or sharing user data
- ai_training: Use of user data for AI/ML training
- retention_specified: Specific data retention periods mentioned
- retention_indefinite: Indefinite or permanent data storage
- deletion_rights: User rights to delete their data
- children_restrictions: Age restrictions or child protection measures

Return JSON in this exact format:
{
  "facts": [
    {
      "quote": "exact verbatim quote from the policy",
      "category": "one of the categories above",
      "confidence": 0.0 to 1.0
    }
  ]
}`, doc.SectionURL, doc.Content)
}

func (e *OpenAIExtractor) parseFactsJSON(content string, baseURL string) ([]ports.Fact, error) {
    var parsed struct {
        Facts []struct {
            Quote      string  `json:"quote"`
            Category   string  `json:"category"`
            Confidence float64 `json:"confidence"`
        } `json:"facts"`
    }

    if err := json.Unmarshal([]byte(content), &parsed); err != nil {
        return nil, fmt.Errorf("parse facts json: %w", err)
    }

    facts := make([]ports.Fact, 0, len(parsed.Facts))
    for _, f := range parsed.Facts {
        // Validate required fields
        if f.Quote == "" {
            continue // Skip facts without quotes
        }

        facts = append(facts, ports.Fact{
            Quote:      f.Quote,
            SectionURL: baseURL,
            Category:   f.Category,
            Confidence: f.Confidence,
        })
    }

    return facts, nil
}

const systemPrompt = `You are a privacy policy analyzer. Extract specific factual claims from privacy policies with exact quotes. Only extract claims you can directly quote from the text. Be conservative and accurate.`
```

#### Ollama Adapter (`ollama.go`)
```go
package ai

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "camille/internal/ports"
)

type OllamaExtractor struct {
    BaseURL        string // e.g., "http://localhost:11434"
    Model          string // e.g., "llama3.2", "mistral"
    Timeout        time.Duration
    PromptVersion  string
    HTTPClient     *http.Client
}

type ollamaRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
    Format string `json:"format"` // "json" for structured output
}

type ollamaResponse struct {
    Response string `json:"response"`
    Done     bool   `json:"done"`
}

func (e *OllamaExtractor) ExtractFacts(ctx context.Context, doc ports.PolicyDoc) ([]ports.Fact, error) {
    prompt := e.buildPrompt(doc)

    reqBody := ollamaRequest{
        Model:  e.Model,
        Prompt: prompt,
        Stream: false,
        Format: "json",
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    endpoint := fmt.Sprintf("%s/api/generate", e.BaseURL)
    req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    client := e.HTTPClient
    if client == nil {
        client = &http.Client{Timeout: e.Timeout}
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("api request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("ollama api error (status %d): %s", resp.StatusCode, string(body))
    }

    var apiResp ollamaResponse
    if err := json.Unmarshal(body, &apiResp); err != nil {
        return nil, fmt.Errorf("unmarshal response: %w", err)
    }

    return e.parseFactsJSON(apiResp.Response, doc.SectionURL)
}

// buildPrompt and parseFactsJSON are identical to OpenAI version
// (reuse the same logic or extract to a shared helper)
```

### Configuration & Initialization

Add to config loading (e.g., `internal/config/config.go` or `cmd/*/main.go`):

```go
type AIConfig struct {
    Enabled  bool          `env:"AI_ENABLED" envDefault:"true"`
    Provider string        `env:"AI_PROVIDER" envDefault:"openai"` // "openai" or "ollama"
    Model    string        `env:"AI_MODEL" envDefault:"gpt-4o-mini"`
    APIKey   string        `env:"AI_API_KEY"`
    BaseURL  string        `env:"AI_BASE_URL" envDefault:"http://localhost:11434"` // for Ollama
    Timeout  time.Duration `env:"AI_TIMEOUT" envDefault:"30s"`
}

func NewAIExtractor(cfg AIConfig) (ports.AIExtractor, error) {
    if !cfg.Enabled {
        return nil, fmt.Errorf("AI extraction is disabled")
    }

    switch cfg.Provider {
    case "openai":
        if cfg.APIKey == "" {
            return nil, fmt.Errorf("AI_API_KEY required for openai provider")
        }
        return &ai.OpenAIExtractor{
            APIKey:  cfg.APIKey,
            Model:   cfg.Model,
            Timeout: cfg.Timeout,
            PromptVersion: "mvp-v1",
        }, nil

    case "ollama":
        return &ai.OllamaExtractor{
            BaseURL: cfg.BaseURL,
            Model:   cfg.Model,
            Timeout: cfg.Timeout,
            PromptVersion: "mvp-v1",
        }, nil

    default:
        return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
    }
}
```

### Integration with PipelineProcessor

The `PipelineProcessor` will use the AI extractor in step 3:

```go
type PipelineProcessor struct {
    JobRepo      ports.JobRepository
    EvidenceRepo ports.EvidenceRepository
    SignalsRepo  ports.SignalsRepository
    ScoreRepo    ports.ScoreRepository
    AIExtractor  ports.AIExtractor // <-- Injected here
    Fetcher      PolicyFetcher
}

func (p *PipelineProcessor) Process(ctx context.Context, scanID string) error {
    // ... Step 1: Fetch & Extract
    // ... Step 2: Chunk & Route

    // Step 3: AI Extraction
    var allFacts []ports.Fact
    for _, chunk := range chunks {
        facts, err := p.AIExtractor.ExtractFacts(ctx, chunk)
        if err != nil {
            // Soft fail: log error and continue
            log.Printf("AI extraction error for %s: %v", chunk.SectionURL, err)
            continue
        }
        allFacts = append(allFacts, facts...)

        // Store raw evidence
        evidencePayload, _ := json.Marshal(facts)
        _, _ = p.EvidenceRepo.AddEvidence(ctx, scanID, "ai_extraction", chunk.SectionURL,
            computeHash(evidencePayload), evidencePayload, map[string]interface{}{
                "model": "gpt-4o-mini", // or get from extractor
                "prompt_version": "mvp-v1",
            })
    }

    // ... Step 4: Normalize to signals
    // ... Step 5: Score
    // ... Step 6: Complete
}
```

### Testing Strategy

1. **Unit Tests** (`internal/adapters/ai/openai_test.go`):
   - Mock HTTP client responses
   - Test JSON parsing with various response formats
   - Test error handling (API errors, malformed JSON, missing quotes)
   - Test prompt generation

2. **Golden Tests** (`internal/adapters/ai/testdata/`):
   - Store sample policy paragraphs as input files
   - Store expected extracted facts as golden JSON files
   - Compare actual extractor output against golden files

3. **Integration Tests**:
   - Use test HTTP server with pre-recorded responses
   - Test end-to-end flow: doc → extractor → facts → signals

### Environment Variables Summary

| Variable | Default | Description |
|----------|---------|-------------|
| `AI_ENABLED` | `true` | Enable/disable AI extraction |
| `AI_PROVIDER` | `openai` | Provider: `openai` or `ollama` |
| `AI_MODEL` | `gpt-4o-mini` | Model name (provider-specific) |
| `AI_API_KEY` | (none) | API key for OpenAI (required if provider=openai) |
| `AI_BASE_URL` | `http://localhost:11434` | Base URL for Ollama |
| `AI_TIMEOUT` | `30s` | Request timeout for AI calls |

### Error Handling & Resilience

- **Timeouts**: Respect `AI_TIMEOUT` via context cancellation
- **Rate Limits**: Log and soft-fail; continue processing other chunks
- **Malformed JSON**: Log warning, drop invalid facts, continue
- **Missing Quotes**: Skip facts that don't have required `quote` field
- **Network Errors**: Soft-fail individual chunks; don't fail entire scan

### Security Considerations

- **API Key Storage**: Never log or expose `AI_API_KEY` in responses
- **Input Sanitization**: Limit doc content size (already capped at 1MB by fetcher)
- **Output Validation**: Validate all JSON fields before persisting to DB
- **Prompt Injection**: Use structured JSON mode to reduce risk; validate categories against whitelist

## Config (MVP)
- Add envs: `AI_ENABLED` (default true), `AI_PROVIDER`, `AI_MODEL`, `AI_API_KEY` (if needed), `AI_TIMEOUT`.
- Keep all existing envs unchanged; no impact to current server startup.

## Tests (MVP)
- Golden tests: one paragraph per signal → expected normalized signals.
- Integration: httptest policy page → `POST /scan?wait=true` → verify evidence stored, signals present, and `/profiles/{domain}` returns the computed score.

## Acceptance Checklist
- [ ] New migration creates `evidence` and `signals` tables only; existing tables untouched.
- [ ] Ports and pg adapters added for Evidence and Signals; ScoreRepository supports upsert with `method_version`.
- [ ] `PipelineProcessor` replaces NoopProcessor and is injected into API and workers without changing handler signatures.
- [ ] Fetcher is SSRF‑safe (same eTLD+1, block private IPs), enforces size/time limits, and extracts readable text.
- [ ] AI extractor returns facts; raw response stored in `evidence` with model and prompt metadata.
- [ ] Facts validate (quote + section_url) and normalize to the 6 MVP signals; duplicates collapsed.
- [ ] Privacy score computed deterministically; `scores.method_version` set to `mvp-v1`; `overall = privacy`; `/profiles/{domain}` returns these scores.
- [ ] `POST /scan?wait=true` returns 200 completed with non‑zero privacy score; `/scans/{id}` reflects progress.
- [ ] Basic tests pass (golden + one integration), no OpenAPI changes required.

## Out of Scope (MVP)
- Multi‑locale, PDFs/OCR, headless rendering.
- External sources (ToS;DR, Observatory, OpenCorporates) and summarizer UI.
