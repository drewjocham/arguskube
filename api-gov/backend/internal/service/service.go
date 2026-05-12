package service

import (
	"context"
	"log/slog"

	"github.com/argus/api-gov/internal/models"
)

type SpecService struct {
	specStore     SpecStore
	endpointStore EndpointStore
	logger        *slog.Logger
}

type DriftService struct {
	driftStore DriftStore
	specStore  SpecStore
	logger     *slog.Logger
}

type SpecStore interface {
	Create(ctx context.Context, spec *models.APISpec) error
	GetByID(ctx context.Context, id string) (*models.APISpec, error)
	List(ctx context.Context, limit, offset int) ([]*models.APISpec, int, error)
	Delete(ctx context.Context, id string) error
	GDPRDelete(ctx context.Context, specID string) error
	CountUserData(ctx context.Context, specID string) (map[string]int, error)
	GetAnomalyMetrics(ctx context.Context, specID string) (any, error)
}

type EndpointStore interface {
	Upsert(ctx context.Context, ep *models.Endpoint) error
	UpsertBatch(ctx context.Context, endpoints []*models.Endpoint) error
	GetBySpec(ctx context.Context, specID string, limit, offset int) ([]*models.Endpoint, int, error)
	UpdateEmbedding(ctx context.Context, endpointID string, embedding []float32) error
}

type DriftStore interface {
	Create(ctx context.Context, r *models.DriftReport) error
	CreateBatch(ctx context.Context, reports []*models.DriftReport) error
	List(ctx context.Context, specID string, filter *models.DriftFilter) ([]*models.DriftReport, int, error)
	Summary(ctx context.Context, specID string) (*models.DriftSummary, error)
	Resolve(ctx context.Context, id string) error
	VectorSearch(ctx context.Context, embedding []float32, limit int) ([]*models.Endpoint, error)
}

func NewSpecService(specs SpecStore, endpoints EndpointStore, logger *slog.Logger) *SpecService {
	return &SpecService{specStore: specs, endpointStore: endpoints, logger: logger}
}

func (s *SpecService) Create(ctx context.Context, spec *models.APISpec) error {
	return s.specStore.Create(ctx, spec)
}

func (s *SpecService) GetByID(ctx context.Context, id string) (*models.APISpec, error) {
	return s.specStore.GetByID(ctx, id)
}

func (s *SpecService) List(ctx context.Context, limit, offset int) ([]*models.APISpec, int, error) {
	return s.specStore.List(ctx, limit, offset)
}

func (s *SpecService) Delete(ctx context.Context, id string) error {
	return s.specStore.Delete(ctx, id)
}

func (s *SpecService) GetEndpoints(ctx context.Context, specID string, limit, offset int) ([]*models.Endpoint, int, error) {
	return s.endpointStore.GetBySpec(ctx, specID, limit, offset)
}

func NewDriftService(store DriftStore, specStore SpecStore, logger *slog.Logger) *DriftService {
	return &DriftService{driftStore: store, specStore: specStore, logger: logger}
}

func (d *DriftService) GetReports(ctx context.Context, specID string, filter *models.DriftFilter) ([]*models.DriftReport, int, error) {
	return d.driftStore.List(ctx, specID, filter)
}

func (d *DriftService) Summary(ctx context.Context, specID string) (*models.DriftSummary, error) {
	return d.driftStore.Summary(ctx, specID)
}

func (d *DriftService) Resolve(ctx context.Context, reportID string) error {
	return d.driftStore.Resolve(ctx, reportID)
}
