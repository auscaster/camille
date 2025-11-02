package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"

    httpadapter "camille/internal/adapters/http"
    pg "camille/internal/adapters/postgres"
    "camille/internal/config"
    ports "camille/internal/ports"
    profsvc "camille/internal/services/profiles"
    scansvc "camille/internal/services/scanner"
    compsvc "camille/internal/services/companies"
    scanworker "camille/internal/workers/scanrunner"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Printf("warning: %v", err)
    }
    if cfg.DatabaseURL == "" {
        log.Fatal("DATABASE_URL is required for Postgres adapters")
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    db, err := pg.Connect(ctx, cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("db connect error: %v", err)
    }
    defer db.Close()

    // Wire repositories to services (ports)
    var _ ports.DomainRepository = db
    var _ ports.ScanRepository = db
    var _ ports.ScoreRepository = db

    scanner := scansvc.New(db, db)
    profiles := profsvc.New(db)
    companies := compsvc.New()

    processor := scanworker.NoopProcessor{Repo: db}
    srv := httpadapter.New(scanner, profiles, companies, db, processor)
    r := chi.NewRouter()
    r.Mount("/", srv.Routes())

    // graceful shutdown
    // Optional background job workers
    if cfg.ScanWorkers > 0 {
        go scanworker.Run(ctx, db, processor, cfg.ScanWorkers, 500*time.Millisecond)
        log.Printf("scan workers started: %d", cfg.ScanWorkers)
    }

    errCh := make(chan error, 1)
    go func() { errCh <- http.ListenAndServe(cfg.ListenAddr, r) }()
    log.Printf("listening on %s", cfg.ListenAddr)

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    select {
    case sig := <-sigCh:
        log.Printf("shutting down on %s", sig)
        cancel()
        time.Sleep(300 * time.Millisecond)
    case err := <-errCh:
        log.Fatal(fmt.Errorf("server error: %w", err))
    }
}
