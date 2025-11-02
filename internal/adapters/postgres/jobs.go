package postgres

import (
    "context"
    "errors"
    "time"

    "github.com/jackc/pgx/v5"

    "camille/internal/ports"
)

// ClaimNext selects the next queued job using SKIP LOCKED and marks it running.
func (db *DB) ClaimNext(ctx context.Context) (job ports.ScanJob, found bool, err error) {
    // Use explicit transaction to safely lock and transition state
    tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil { return job, false, err }
    defer func() {
        if err != nil { _ = tx.Rollback(ctx) } else { _ = tx.Commit(ctx) }
    }()

    // Lock the next queued job
    err = tx.QueryRow(ctx, `
        SELECT id, scan_id FROM scan_jobs
        WHERE status = 'queued'
        ORDER BY queued_at
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    `).Scan(&job.ID, &job.ScanID)
    if errors.Is(err, pgx.ErrNoRows) {
        return job, false, nil
    }
    if err != nil { return job, false, err }

    // Mark job running and bump attempts
    if _, err = tx.Exec(ctx, `
        UPDATE scan_jobs SET status='running', started_at=now(), attempts=attempts+1 WHERE id=$1
    `, job.ID); err != nil {
        return job, false, err
    }
    // Ensure scans reflects running
    if _, err = tx.Exec(ctx, `
        UPDATE scans SET status='running', started_at=COALESCE(started_at, now()) WHERE id=$1
    `, job.ScanID); err != nil {
        return job, false, err
    }
    return job, true, nil
}

func (db *DB) MarkRunning(ctx context.Context, jobID string) error {
    _, err := db.Pool.Exec(ctx, `UPDATE scan_jobs SET status='running', started_at=COALESCE(started_at, now()) WHERE id=$1`, jobID)
    return err
}

func (db *DB) UpdateScanProgress(ctx context.Context, scanID string, progress float64) error {
    if progress < 0 { progress = 0 }
    if progress > 1 { progress = 1 }
    _, err := db.Pool.Exec(ctx, `UPDATE scans SET progress=$2 WHERE id=$1`, scanID, progress)
    return err
}

func (db *DB) MarkCompleted(ctx context.Context, jobID string) error {
    // complete job and scan atomically
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil { return err }
    defer func() {
        if err != nil { _ = tx.Rollback(ctx) } else { _ = tx.Commit(ctx) }
    }()

    var scanID string
    if err = tx.QueryRow(ctx, `SELECT scan_id FROM scan_jobs WHERE id=$1`, jobID).Scan(&scanID); err != nil {
        return err
    }
    if _, err = tx.Exec(ctx, `UPDATE scan_jobs SET status='completed', finished_at=now() WHERE id=$1`, jobID); err != nil {
        return err
    }
    if _, err = tx.Exec(ctx, `UPDATE scans SET status='completed', progress=1, finished_at=now() WHERE id=$1`, scanID); err != nil {
        return err
    }
    return nil
}

func (db *DB) MarkFailed(ctx context.Context, jobID string, reason string) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil { return err }
    defer func() {
        if err != nil { _ = tx.Rollback(ctx) } else { _ = tx.Commit(ctx) }
    }()
    var scanID string
    if err = tx.QueryRow(ctx, `SELECT scan_id FROM scan_jobs WHERE id=$1`, jobID).Scan(&scanID); err != nil { return err }
    if _, err = tx.Exec(ctx, `UPDATE scan_jobs SET status='failed', finished_at=now() WHERE id=$1`, jobID); err != nil { return err }
    if _, err = tx.Exec(ctx, `UPDATE scans SET status='failed', finished_at=now() WHERE id=$1`, scanID); err != nil { return err }
    return nil
}

// StartJobForScan marks the job for a specific scan as running and returns the job id.
func (db *DB) StartJobForScan(ctx context.Context, scanID string) (string, error) {
    tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil { return "", err }
    defer func() {
        if err != nil { _ = tx.Rollback(ctx) } else { _ = tx.Commit(ctx) }
    }()

    var jobID string
    // lock specific job row if queued
    err = tx.QueryRow(ctx, `
        SELECT id FROM scan_jobs
        WHERE scan_id = $1 AND status = 'queued'
        FOR UPDATE SKIP LOCKED
    `, scanID).Scan(&jobID)
    if err != nil { return "", err }
    if _, err = tx.Exec(ctx, `UPDATE scan_jobs SET status='running', started_at=now(), attempts=attempts+1 WHERE id=$1`, jobID); err != nil {
        return "", err
    }
    if _, err = tx.Exec(ctx, `UPDATE scans SET status='running', started_at=COALESCE(started_at, now()) WHERE id=$1`, scanID); err != nil {
        return "", err
    }
    return jobID, nil
}
