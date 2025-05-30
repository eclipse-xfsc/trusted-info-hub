// Code generated by goa v3.20.1, DO NOT EDIT.
//
// health endpoints
//
// Command:
// $ goa gen github.com/eclipse-xfsc/trusted-info-hub/design

package health

import (
	"context"

	goa "goa.design/goa/v3/pkg"
)

// Endpoints wraps the "health" service endpoints.
type Endpoints struct {
	Liveness  goa.Endpoint
	Readiness goa.Endpoint
}

// NewEndpoints wraps the methods of the "health" service with endpoints.
func NewEndpoints(s Service) *Endpoints {
	return &Endpoints{
		Liveness:  NewLivenessEndpoint(s),
		Readiness: NewReadinessEndpoint(s),
	}
}

// Use applies the given middleware to all the "health" service endpoints.
func (e *Endpoints) Use(m func(goa.Endpoint) goa.Endpoint) {
	e.Liveness = m(e.Liveness)
	e.Readiness = m(e.Readiness)
}

// NewLivenessEndpoint returns an endpoint function that calls the method
// "Liveness" of service "health".
func NewLivenessEndpoint(s Service) goa.Endpoint {
	return func(ctx context.Context, req any) (any, error) {
		return s.Liveness(ctx)
	}
}

// NewReadinessEndpoint returns an endpoint function that calls the method
// "Readiness" of service "health".
func NewReadinessEndpoint(s Service) goa.Endpoint {
	return func(ctx context.Context, req any) (any, error) {
		return s.Readiness(ctx)
	}
}
