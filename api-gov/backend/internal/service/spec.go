package service

import (
	"context"
	"fmt"

	"github.com/argus/api-gov/internal/apperrors"
	"github.com/argus/api-gov/internal/models"
)

type CreateSpecRequest struct {
	Name    string
	Version string
	Content []byte
	Format  string
}

func (s *SpecService) GDPRDelete(ctx context.Context, specID string) error {
	return s.specStore.GDPRDelete(ctx, specID)
}

func (s *SpecService) CountUserData(ctx context.Context, specID string) (map[string]int, error) {
	return s.specStore.CountUserData(ctx, specID)
}

func (s *SpecService) GetAnomalyMetrics(ctx context.Context, specID string) (any, error) {
	return s.specStore.GetAnomalyMetrics(ctx, specID)
}

func (s *SpecService) CreateSpec(ctx context.Context, req CreateSpecRequest) (*models.APISpec, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("%w: spec name is required", apperrors.ErrValidation)
	}
	if len(req.Content) == 0 {
		return nil, fmt.Errorf("%w: spec content is required", apperrors.ErrValidation)
	}
	if req.Version == "" {
		req.Version = "1.0.0"
	}
	if req.Format == "" {
		req.Format = "openapi_3_1"
	}

	spec := &models.APISpec{
		Name:    req.Name,
		Version: req.Version,
		Content: req.Content,
		Format:  req.Format,
	}

	if err := s.specStore.Create(ctx, spec); err != nil {
		return nil, fmt.Errorf("create spec: %w", err)
	}

	return spec, nil
}
