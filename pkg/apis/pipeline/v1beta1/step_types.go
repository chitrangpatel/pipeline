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

package v1beta1

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genclient:noStatus
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Step represents a collection of sequential steps that are run as part of a
// Pipeline using a set of inputs and producing a set of outputs. Steps execute
// when StepRuns are created that provide the input parameters and resources and
// output resources the Step requires.
//
// +k8s:openapi-gen=true
type MyStep struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Step from the client
	// +optional
	Spec Step `json:"spec"`
}

var _ kmeta.OwnerRefable = (*MyStep)(nil)

// MyStep returns the task's spec
func (s *MyStep) MyStepSpec() Step {
	return s.Spec
}

// MyStepMetadata returns the task's ObjectMeta
func (s *MyStep) MyStepMetadata() metav1.ObjectMeta {
	return s.ObjectMeta
}

// Copy returns a deep copy of the task
func (s *MyStep) Copy() MyStepObject {
	return s.DeepCopy()
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*MyStep) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.StepControllerName)
}

// StepList contains a list of Step
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MyStepList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MyStep `json:"items"`
}
