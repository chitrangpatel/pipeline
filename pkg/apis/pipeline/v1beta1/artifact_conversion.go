package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (a Artifacts) convertTo(ctx context.Context, sink *v1.Artifacts) {
	sink.Inputs = nil
	for _, ia := range a.Inputs {
		new := v1.Artifact{}
		ia.convertTo(ctx, &new)
		sink.Inputs = append(sink.Inputs, new)
	}
	sink.Outputs = nil
	for _, oa := range a.Outputs {
		new := v1.Artifact{}
		oa.convertTo(ctx, &new)
		sink.Outputs = append(sink.Outputs, new)
	}
}

func (a *Artifacts) convertFrom(ctx context.Context, source *v1.Artifacts) {
	a.Inputs = nil
	for _, ia := range source.Inputs {
		new := Artifact{}
		new.convertFrom(ctx, ia)
		a.Inputs = append(a.Inputs, new)
	}
	a.Outputs = nil
	for _, oa := range source.Outputs {
		new := Artifact{}
		new.convertFrom(ctx, oa)
		a.Outputs = append(a.Outputs, new)
	}
}

func (a Artifact) convertTo(ctx context.Context, sink *v1.Artifact) {
	sink.Name = a.Name
	sink.Type = a.Type
	sink.Description = a.Description
	newValue := v1.ParamValue{}
	a.Value.convertTo(ctx, &newValue)
	sink.Value = newValue
	if a.TaskRef != nil {
		sink.TaskRef = &v1.TaskRef{}
		a.TaskRef.convertTo(ctx, sink.TaskRef)
	}
}

func (a *Artifact) convertFrom(ctx context.Context, source v1.Artifact) {
	a.Name = source.Name
	a.Type = source.Type
	a.Description = source.Description
	newValue := ParamValue{}
	newValue.convertFrom(ctx, source.Value)
	a.Value = newValue
	if source.TaskRef != nil {
		newTaskRef := TaskRef{}
		newTaskRef.convertFrom(ctx, *source.TaskRef)
		a.TaskRef = &newTaskRef
	}
}
