package postgres

import (
    "context"
    "errors"
    "strings"

    "github.com/jackc/pgx/v5"
)

// DomainRepository
func (db *DB) GetOrCreate(ctx context.Context, registrable string) (string, error) {
    registrable = strings.ToLower(registrable)
    var id string
    err := db.Pool.QueryRow(ctx, `
        INSERT INTO domains (registrable_domain)
        VALUES ($1)
        ON CONFLICT (registrable_domain) DO UPDATE SET registrable_domain = EXCLUDED.registrable_domain
        RETURNING id
    `, registrable).Scan(&id)
    return id, err
}

// ScanRepository
func (db *DB) Create(ctx context.Context, domainID string, url string) (string, error) {
    var scanID string
    err := db.Pool.QueryRow(ctx, `
        INSERT INTO scans (domain_id, url, status, progress)
        VALUES ($1, $2, 'queued', 0)
        RETURNING id
    `, domainID, url).Scan(&scanID)
    if err != nil {
        return "", err
    }
    // create job row
    _, err = db.Pool.Exec(ctx, `INSERT INTO scan_jobs (scan_id) VALUES ($1)`, scanID)
    return scanID, err
}

func (db *DB) Status(ctx context.Context, scanID string) (string, float64, error) {
    var status string
    var progress float64
    err := db.Pool.QueryRow(ctx, `SELECT status, progress FROM scans WHERE id = $1`, scanID).Scan(&status, &progress)
    if errors.Is(err, pgx.ErrNoRows) {
        return "", 0, ErrNotFound
    }
    return status, progress, err
}

// ScoreRepository
func (db *DB) GetLatestByDomain(ctx context.Context, registrable string) (bool, struct{
    Privacy, Security, Governance, Esg, Overall int
    Badges []string
}, error) {
    var out struct{
        Privacy, Security, Governance, Esg, Overall int
        Badges []string
    }
    var exists bool
    err := db.Pool.QueryRow(ctx, `
        SELECT s.privacy, s.security, s.governance, s.esg, s.overall, COALESCE(s.badges, '[]'::jsonb)
        FROM scores s
        JOIN domains d ON d.id = s.domain_id
        WHERE d.registrable_domain = $1
    `, strings.ToLower(registrable)).Scan(&out.Privacy, &out.Security, &out.Governance, &out.Esg, &out.Overall, &out.Badges)
    if errors.Is(err, pgx.ErrNoRows) {
        return false, out, nil
    }
    if err != nil {
        return false, out, err
    }
    exists = true
    return exists, out, nil
}

var ErrNotFound = errString("not found")
type errString string
func (e errString) Error() string { return string(e) }

