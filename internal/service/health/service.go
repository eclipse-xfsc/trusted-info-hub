package health

import (
	"context"

	"github.com/eclipse-xfsc/trusted-info-hub/gen/health"
)

type Service struct {
	version string
}

func New(version string) *Service {
	return &Service{version: version}
}

func (s *Service) Liveness(_ context.Context) (*health.HealthResponse, error) {
	return &health.HealthResponse{
		Service: "infohub",
		Status:  "up",
		Version: s.version,
	}, nil
}

func (s *Service) Readiness(_ context.Context) (*health.HealthResponse, error) {
	return &health.HealthResponse{
		Service: "infohub",
		Status:  "up",
		Version: s.version,
	}, nil
}
