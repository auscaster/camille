package scanrunner

import (
    "context"
    "log"
    "time"

    "camille/internal/ports"
)

// ScanProcessor performs the scan work for a job's scan id.
type ScanProcessor interface {
    Process(ctx context.Context, scanID string) error
}

// NoopProcessor marks scans as completed without real work. Replace with real pipeline.
type NoopProcessor struct{ Repo ports.JobRepository }

func (n NoopProcessor) Process(ctx context.Context, scanID string) error {
    // Simulate incremental progress
    for p := 0.0; p < 1.0; p += 0.25 {
        if err := n.Repo.UpdateScanProgress(ctx, scanID, p); err != nil { return err }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(150 * time.Millisecond):
        }
    }
    return n.Repo.UpdateScanProgress(ctx, scanID, 1.0)
}

// Run starts worker goroutines that claim jobs and process them.
func Run(ctx context.Context, repo ports.JobRepository, processor ScanProcessor, concurrency int, pollInterval time.Duration) {
    if concurrency < 1 { return }
    jobsCh := make(chan ports.ScanJob, concurrency)

    // dispatcher loop
    go func() {
        ticker := time.NewTicker(pollInterval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                close(jobsCh)
                return
            case <-ticker.C:
                for {
                    job, found, err := repo.ClaimNext(ctx)
                    if err != nil {
                        log.Printf("job claim error: %v", err)
                        break
                    }
                    if !found { break }
                    jobsCh <- ports.ScanJob{ID: job.ID, ScanID: job.ScanID}
                }
            }
        }
    }()

    // workers
    for i := 0; i < concurrency; i++ {
        go func(idx int) {
            for job := range jobsCh {
                if err := processor.Process(ctx, job.ScanID); err != nil {
                    _ = repo.MarkFailed(ctx, job.ID, err.Error())
                    log.Printf("worker %d: job %s failed: %v", idx, job.ID, err)
                    continue
                }
                if err := repo.MarkCompleted(ctx, job.ID); err != nil {
                    log.Printf("worker %d: complete err: %v", idx, err)
                }
            }
        }(i)
    }
}

// ProcessInline starts and processes a specific scan synchronously using the same processor logic
// as the background workers. It marks the job as running, calls processor.Process, and completes or fails.
func ProcessInline(ctx context.Context, repo ports.JobRepository, processor ScanProcessor, scanID string) error {
    jobID, err := repo.StartJobForScan(ctx, scanID)
    if err != nil { return err }
    if err := processor.Process(ctx, scanID); err != nil {
        _ = repo.MarkFailed(ctx, jobID, err.Error())
        return err
    }
    return repo.MarkCompleted(ctx, jobID)
}
