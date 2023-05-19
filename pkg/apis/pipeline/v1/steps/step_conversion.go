package steps

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*Step)(nil)

// ConvertTo implements apis.Convertible
func (s *Step) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *Step:
		sink.ObjectMeta = s.ObjectMeta
		return s.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ss *StepSpec) ConvertTo(ctx context.Context, sink *StepSpec) error {
	sink.Name = ss.Name
	sink.Image = ss.Image
	sink.Command = ss.Command
	sink.Args = ss.Args
	sink.WorkingDir = ss.WorkingDir
	sink.EnvFrom = ss.EnvFrom
	sink.Env = ss.Env
	sink.Script = ss.Script
	return nil
}

// ConvertFrom implements apis.Convertible
func (s *Step) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *Step:
		s.ObjectMeta = source.ObjectMeta
		return s.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", s)
	}
}

// ConvertFrom implements apis.Convertible
func (ss *StepSpec) ConvertFrom(ctx context.Context, source *StepSpec) error {
	ss.Name = source.Name
	ss.Image = source.Image
	ss.Command = source.Command
	ss.Args = source.Args
	ss.WorkingDir = source.WorkingDir
	ss.EnvFrom = source.EnvFrom
	ss.Env = source.Env
	ss.Script = source.Script
	return nil
}
