package domain

import "time"

// Core domain models used internally. API types are generated from OpenAPI and
// sit in internal/api; keep these decoupled where helpful.

type Domain struct {
    ID               string
    RegistrableDomain string
    CompanyRef       *string
    FirstSeenAt      time.Time
}

type Company struct {
    ID                  string
    LegalName           string
    Jurisdiction        string
    RegistryID          *string // e.g., opencorporates id
    LEI                 *string
    Website             *string
    Confidence          float64
}

type Scan struct {
    ID         string
    DomainRef  string
    StartedAt  *time.Time
    FinishedAt *time.Time
    Status     string // queued|running|completed|failed
}

type Evidence struct {
    ID         string
    ScanRef    string
    SourceType string
    SourceURL  *string
    RetrievedAt time.Time
    Hash       string
    Payload    any
}

type Signal struct {
    ID         string
    DomainRef  string
    Code       string
    Value      any
    Severity   string
    Confidence float64
    Source     string
    RetrievedAt time.Time
}

type Issue struct {
    ID       string
    ScanRef  string
    Code     string
    Severity string
    Summary  string
    EvidenceRefs []string
}

type Score struct {
    ID         string
    DomainRef  string
    Privacy    int
    Security   int
    Governance int
    ESG        int
    Overall    int
    Badges     []string
    ComputedAt time.Time
}

