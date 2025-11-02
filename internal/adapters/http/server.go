package httpadapter

import (
    "context"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    api "camille/internal/api"
    "camille/internal/ports"
    profilesvc "camille/internal/services/profiles"
    scanrunner "camille/internal/workers/scanrunner"
)

// Server implements the generated StrictServerInterface.
type Server struct {
    scanner   ports.Scanner
    profiles  ports.Profiles
    companies ports.Companies
    jobs      ports.JobRepository
    processor scanrunner.ScanProcessor
}

func New(scanner ports.Scanner, profiles ports.Profiles, companies ports.Companies, jobs ports.JobRepository, processor scanrunner.ScanProcessor) *Server {
    return &Server{scanner: scanner, profiles: profiles, companies: companies, jobs: jobs, processor: processor}
}

// Routes returns a chi.Router mounting the generated handlers.
func (s *Server) Routes() chi.Router {
    r := chi.NewRouter()
    // Generated handler wiring
    handler := api.NewStrictHandler(s, nil)
    api.HandlerFromMux(handler, r)
    return r
}

// Strict handler methods

func (s *Server) GetHealthz(ctx context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
    ok := "ok"
    return api.GetHealthz200JSONResponse{Status: &ok}, nil
}

func (s *Server) PostScan(ctx context.Context, req api.PostScanRequestObject) (api.PostScanResponseObject, error) {
    if req.Body == nil {
        return nil, &runtimeError{code: http.StatusBadRequest, msg: "missing body"}
    }
    id, err := s.scanner.Enqueue(ctx, req.Body.Url)
    if err != nil {
        return nil, err
    }
    // Blocking path for testing
    wait := false
    if req.Params.Wait != nil { wait = *req.Params.Wait }
    if wait {
        // Apply optional timeout
        timeout := 30
        if req.Params.Timeout != nil && *req.Params.Timeout > 0 { timeout = *req.Params.Timeout }
        ctx2, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
        defer cancel()
        // Use the same processor the workers use to keep logic DRY
        if err := scanrunner.ProcessInline(ctx2, s.jobs, s.processor, id); err != nil {
            return nil, err
        }
        status, progress, err := s.scanner.Status(ctx2, id)
        if err != nil { return nil, err }
        fp := float32(progress)
        resp := api.ScanResponse{Id: id, Status: api.ScanStatus(status), Progress: &fp}
        return api.PostScan200JSONResponse(resp), nil
    }
    res := api.ScanAcceptedResponse{ScanId: id}
    return api.PostScan202JSONResponse(res), nil
}

func (s *Server) GetScansId(ctx context.Context, req api.GetScansIdRequestObject) (api.GetScansIdResponseObject, error) {
    status, progress, err := s.scanner.Status(ctx, req.Id)
    if err != nil {
        return nil, err
    }
    fp := float32(progress)
    resp := api.ScanResponse{Id: req.Id, Status: api.ScanStatus(status), Progress: &fp}
    return api.GetScansId200JSONResponse(resp), nil
}

func (s *Server) GetProfilesDomain(ctx context.Context, req api.GetProfilesDomainRequestObject) (api.GetProfilesDomainResponseObject, error) {
    prof, err := s.profiles.GetLatest(ctx, req.Domain)
    if err != nil {
        if err == profilesvc.ErrNotFound {
            return api.GetProfilesDomain404Response{}, nil
        }
        return nil, err
    }
    // prof is assumed to be already in API shape
    return api.GetProfilesDomain200JSONResponse(prof.(api.Profile)), nil
}

func (s *Server) GetCompaniesOpencorporatesId(ctx context.Context, req api.GetCompaniesOpencorporatesIdRequestObject) (api.GetCompaniesOpencorporatesIdResponseObject, error) {
    ident, err := s.companies.GetIdentity(ctx, req.OpencorporatesId)
    if err != nil {
        return nil, err
    }
    return api.GetCompaniesOpencorporatesId200JSONResponse(ident.(api.CompanyIdentity)), nil
}

type runtimeError struct{ code int; msg string }
func (e *runtimeError) Error() string { return e.msg }
