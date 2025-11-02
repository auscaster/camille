package ports

import "context"

type ScanJob struct {
    ID     string
    ScanID string
}

// JobRepository supports claiming and updating scan jobs.
type JobRepository interface {
    ClaimNext(ctx context.Context) (job ScanJob, found bool, err error)
    MarkRunning(ctx context.Context, jobID string) error
    UpdateScanProgress(ctx context.Context, scanID string, progress float64) error
    MarkCompleted(ctx context.Context, jobID string) error
    MarkFailed(ctx context.Context, jobID string, reason string) error
    StartJobForScan(ctx context.Context, scanID string) (jobID string, err error)
}
