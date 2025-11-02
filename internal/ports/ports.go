package ports

import "context"

// Scanner enqueues and tracks scans.
type Scanner interface {
    Enqueue(ctx context.Context, url string) (scanID string, err error)
    Status(ctx context.Context, scanID string) (status string, progress float64, err error)
}

// Profiles provides latest profiles for domains.
type Profiles interface {
    GetLatest(ctx context.Context, domain string) (any, error)
}

// Companies provides normalized identity snapshots.
type Companies interface {
    GetIdentity(ctx context.Context, ocID string) (any, error)
}
