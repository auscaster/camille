package companies

import (
    "context"
    api "camille/internal/api"
)

// Placeholder service; wire to OpenCorporates repo later.
type Service struct{}

func New() *Service { return &Service{} }

func (s *Service) GetIdentity(ctx context.Context, ocID string) (any, error) {
    ident := api.CompanyIdentity{OpencorporatesId: ocID, Name: "Unknown"}
    return ident, nil
}

