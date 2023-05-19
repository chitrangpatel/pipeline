/*
Copyright 2019 The Tekton Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package steps

import (
	"context"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ apis.Validatable = (*Step)(nil)
var _ resourcesemantics.VerbLimited = (*Step)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (s *Step) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate implements apis.Validatable
func (s *Step) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs
}

// Validate implements apis.Validatable
func (ss *StepSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs
}
