package profiles

import (
    "context"

    api "camille/internal/api"
    "camille/internal/ports"
)

type Service struct {
    scores ports.ScoreRepository
}

func New(scores ports.ScoreRepository) *Service { return &Service{scores: scores} }

func (s *Service) GetLatest(ctx context.Context, domain string) (any, error) {
    exists, score, err := s.scores.GetLatestByDomain(ctx, domain)
    if err != nil {
        return nil, err
    }
    if !exists {
        return nil, ErrNotFound
    }
    prof := api.Profile{
        Domain:  domain,
        Overall: score.Overall,
        Scores:  api.Scores{Privacy: score.Privacy, Security: score.Security, Governance: score.Governance, Esg: score.Esg},
        Badges:  &score.Badges,
    }
    return prof, nil
}

var ErrNotFound = errString("not found")
type errString string
func (e errString) Error() string { return string(e) }

