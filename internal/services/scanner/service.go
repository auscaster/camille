package scanner

import (
    "context"
    "net/url"

    "golang.org/x/net/publicsuffix"

    "camille/internal/ports"
)

type Service struct {
    domains ports.DomainRepository
    scans   ports.ScanRepository
}

func New(domains ports.DomainRepository, scans ports.ScanRepository) *Service {
    return &Service{domains: domains, scans: scans}
}

func (s *Service) Enqueue(ctx context.Context, rawurl string) (string, error) {
    u, err := url.Parse(rawurl)
    if err != nil {
        return "", err
    }
    host := u.Hostname()
    registrable, err := publicsuffix.EffectiveTLDPlusOne(host)
    if err != nil {
        registrable = host
    }
    domainID, err := s.domains.GetOrCreate(ctx, registrable)
    if err != nil {
        return "", err
    }
    scanID, err := s.scans.Create(ctx, domainID, rawurl)
    if err != nil {
        return "", err
    }
    return scanID, nil
}

func (s *Service) Status(ctx context.Context, scanID string) (string, float64, error) {
    return s.scans.Status(ctx, scanID)
}

