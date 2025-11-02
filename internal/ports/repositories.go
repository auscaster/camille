package ports

import "context"

// DomainRepository stores and fetches domains by registrable domain (eTLD+1).
type DomainRepository interface {
    GetOrCreate(ctx context.Context, registrable string) (domainID string, err error)
}

// ScanRepository manages scan records and job tracking.
type ScanRepository interface {
    Create(ctx context.Context, domainID string, url string) (scanID string, err error)
    Status(ctx context.Context, scanID string) (status string, progress float64, err error)
}

// ScoreRepository provides latest score aggregates per domain.
type ScoreRepository interface {
    GetLatestByDomain(ctx context.Context, registrable string) (exists bool, score struct{
        Privacy, Security, Governance, Esg, Overall int
        Badges []string
    }, err error)
}

