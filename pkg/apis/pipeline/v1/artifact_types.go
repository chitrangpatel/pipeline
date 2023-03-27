/*
Copyright 2022 The Tekton Authors
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

package v1

// TaskResult used to describe the results of a task
type Artifacts struct {
	Inputs  []Artifact `json:"inputs,omitempty"`
	Outputs []Artifact `json:"outputs,omitempty"`
}
type Artifact struct {
	// Name the given name
	Name string `json:"name"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description,omitempty"`

	// Value the given value of the result
	Value ParamValue `json:"value,omitempty"`

	TaskRef *TaskRef `json:"taskRef,omitempty"`

	Type string `json:"type,omitempty"`
}

type ArtifactiValue struct {
	Uri    string `json:"uri"`
	Digest string `json:"digest"`
}
