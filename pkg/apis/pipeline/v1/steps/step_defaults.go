package steps

import (
	"context"

	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*Step)(nil)

// SetDefaults implements apis.Defaultable
func (s *Step) SetDefaults(ctx context.Context) {
	// s.Spec.SetDefaults(ctx)
}
