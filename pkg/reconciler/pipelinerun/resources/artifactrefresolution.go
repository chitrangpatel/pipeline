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

package resources

import (
	"fmt"
	"sort"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// ResolvedArtifactRefs represents all of the ResolvedArtifactRef for a pipeline task
type ResolvedArtifactRefs []*ResolvedArtifactRef

// ResolvedArtifactRef represents a artifact ref reference that has been fully resolved (value has been populated).
// If the value is from a Artifact, then the ArtifactReference will be populated to point to the ArtifactReference
// which artifacted in the value
type ResolvedArtifactRef struct {
	Value             v1beta1.ParamValue
	ArtifactReference v1beta1.ArtifactRef
	FromTaskRun       string
	FromRun           string
}

// ResolveArtifactRef resolves any ArtifactReference that are found in the target ResolvedPipelineTask
func ResolveArtifactRef(pipelineRunState PipelineRunState, target *ResolvedPipelineTask) (ResolvedArtifactRefs, string, error) {
	resolvedArtifactRefs, pt, err := convertToArtifactRefs(pipelineRunState, target)
	if err != nil {
		return nil, pt, err
	}
	return validateArrayArtifactsIndex(removeDupArtifacts(resolvedArtifactRefs))
}

// ResolveArtifactRefs resolves any ArtifactReference that are found in the target ResolvedPipelineTask
func ResolveArtifactRefs(pipelineRunState PipelineRunState, targets PipelineRunState) (ResolvedArtifactRefs, string, error) {
	var allResolvedArtifactRefs ResolvedArtifactRefs
	for _, target := range targets {
		resolvedArtifactRefs, pt, err := convertToArtifactRefs(pipelineRunState, target)
		if err != nil {
			return nil, pt, err
		}
		allResolvedArtifactRefs = append(allResolvedArtifactRefs, resolvedArtifactRefs...)
	}
	return validateArrayArtifactsIndex(removeDupArtifacts(allResolvedArtifactRefs))
}

// validateArrayArtifactsIndex checks if the artifact array indexing reference is out of bound of the array size
func validateArrayArtifactsIndex(allResolvedArtifactRefs ResolvedArtifactRefs) (ResolvedArtifactRefs, string, error) {
	for _, r := range allResolvedArtifactRefs {
		if r.Value.Type == v1beta1.ParamTypeArray {
			if r.ArtifactReference.ArtifactsIndex >= len(r.Value.ArrayVal) {
				return nil, "", fmt.Errorf("Array Artifact Index %d for Task %s Artifact %s is out of bound of size %d", r.ArtifactReference.ArtifactsIndex, r.ArtifactReference.PipelineTask, r.ArtifactReference.Artifact, len(r.Value.ArrayVal))
			}
		}
	}
	return allResolvedArtifactRefs, "", nil
}

// extractArtifactRefs resolves any ArtifactReference that are found in param or pipeline artifact
// Returns nil if none are found
func extractArtifactRefsForParam(pipelineRunState PipelineRunState, param v1beta1.Param) (ResolvedArtifactRefs, error) {
	expressions, ok := v1beta1.GetVarSubstitutionExpressionsForParam(param)
	if ok {
		return extractArtifactRefs(expressions, pipelineRunState)
	}
	return nil, nil
}

func extractArtifactRefs(expressions []string, pipelineRunState PipelineRunState) (ResolvedArtifactRefs, error) {
	artifactRefs := v1beta1.NewArtifactRefs(expressions)
	var resolvedArtifactRefs ResolvedArtifactRefs
	for _, artifactRef := range artifactRefs {
		resolvedArtifactRef, _, err := resolveArtifactRef(pipelineRunState, artifactRef)
		if err != nil {
			return nil, err
		}
		resolvedArtifactRefs = append(resolvedArtifactRefs, resolvedArtifactRef)
	}
	return removeDupArtifacts(resolvedArtifactRefs), nil
}

func removeDupArtifacts(refs ResolvedArtifactRefs) ResolvedArtifactRefs {
	if refs == nil {
		return nil
	}
	resolvedArtifactRefByRef := make(map[v1beta1.ArtifactRef]*ResolvedArtifactRef, len(refs))
	for _, resolvedArtifactRef := range refs {
		resolvedArtifactRefByRef[resolvedArtifactRef.ArtifactReference] = resolvedArtifactRef
	}
	deduped := make([]*ResolvedArtifactRef, 0, len(resolvedArtifactRefByRef))

	// Sort the artifacting keys to produce a deterministic ordering.
	order := make([]v1beta1.ArtifactRef, 0, len(refs))
	for key := range resolvedArtifactRefByRef {
		order = append(order, key)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].PipelineTask > order[j].PipelineTask {
			return false
		}
		if order[i].Artifact > order[j].Artifact {
			return false
		}
		return true
	})

	for _, key := range order {
		deduped = append(deduped, resolvedArtifactRefByRef[key])
	}
	return deduped
}

// convertToArtifactRefs walks a PipelineTask looking for artifact references. If any are
// found they are resolved to a value by searching pipelineRunState. The list of resolved
// references are returned. If an error is encountered due to an invalid artifact reference
// then a nil list and error is returned instead.
func convertToArtifactRefs(pipelineRunState PipelineRunState, target *ResolvedPipelineTask) (ResolvedArtifactRefs, string, error) {
	var resolvedArtifactRefs ResolvedArtifactRefs
	for _, ref := range v1beta1.PipelineTaskArtifactRefs(target.PipelineTask) {
		resolved, pt, err := resolveArtifactRef(pipelineRunState, ref)
		if err != nil {
			return nil, pt, err
		}
		resolvedArtifactRefs = append(resolvedArtifactRefs, resolved)
	}
	return resolvedArtifactRefs, "", nil
}

func resolveArtifactRef(pipelineState PipelineRunState, artifactRef *v1beta1.ArtifactRef) (*ResolvedArtifactRef, string, error) {
	referencedPipelineTask := pipelineState.ToMap()[artifactRef.PipelineTask]
	if referencedPipelineTask == nil {
		return nil, artifactRef.PipelineTask, fmt.Errorf("could not find task %q referenced by artifact", artifactRef.PipelineTask)
	}
	if !referencedPipelineTask.isSuccessful() {
		return nil, artifactRef.PipelineTask, fmt.Errorf("task %q referenced by artifact was not successful", referencedPipelineTask.PipelineTask.Name)
	}

	var runName, taskRunName string
	var artifactValue v1beta1.ParamValue
	var err error
	taskRunName = referencedPipelineTask.TaskRun.Name
	artifactValue, err = findTaskArtifactForParam(referencedPipelineTask.TaskRun, artifactRef)
	if err != nil {
		return nil, artifactRef.PipelineTask, err
	}

	return &ResolvedArtifactRef{
		Value:             artifactValue,
		FromTaskRun:       taskRunName,
		FromRun:           runName,
		ArtifactReference: *artifactRef,
	}, "", nil
}

func findTaskArtifactForParam(taskRun *v1beta1.TaskRun, reference *v1beta1.ArtifactRef) (v1beta1.ParamValue, error) {
	artifacts := taskRun.Status.TaskRunStatusFields.Artifacts
	for _, artifact := range artifacts.Outputs {
		if artifact.Name == reference.Artifact {
			return artifact.Value, nil
		}
	}
	return v1beta1.ParamValue{}, fmt.Errorf("Could not find artifact with name %s for task %s", reference.Artifact, reference.PipelineTask)
}

func (rs ResolvedArtifactRefs) getStringReplacements() map[string]string {
	replacements := map[string]string{}
	for _, r := range rs {
		switch r.Value.Type {
		case v1beta1.ParamTypeArray:
			for i := 0; i < len(r.Value.ArrayVal); i++ {
				for _, target := range r.getReplaceTargetfromArrayIndex(i) {
					replacements[target] = r.Value.ArrayVal[i]
				}
			}
		case v1beta1.ParamTypeObject:
			for key, element := range r.Value.ObjectVal {
				for _, target := range r.getReplaceTargetfromObjectKey(key) {
					replacements[target] = element
				}
			}

		default:
			for _, target := range r.getReplaceTarget() {
				replacements[target] = r.Value.StringVal
			}
		}
	}
	return replacements
}

func (rs ResolvedArtifactRefs) getArrayReplacements() map[string][]string {
	replacements := map[string][]string{}
	for _, r := range rs {
		if r.Value.Type == v1beta1.ParamType(v1beta1.ParamTypeArray) {
			for _, target := range r.getReplaceTarget() {
				replacements[target] = r.Value.ArrayVal
			}
		}
	}
	return replacements
}

func (rs ResolvedArtifactRefs) getObjectReplacements() map[string]map[string]string {
	replacements := map[string]map[string]string{}
	for _, r := range rs {
		if r.Value.Type == v1beta1.ParamType(v1beta1.ParamTypeObject) {
			for _, target := range r.getReplaceTarget() {
				replacements[target] = r.Value.ObjectVal
			}
		}
	}
	return replacements
}

func (r *ResolvedArtifactRef) getReplaceTarget() []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.outputs.%s", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact),
		fmt.Sprintf("%s.%s.%s.outputs[%q]", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact),
		fmt.Sprintf("%s.%s.%s.outputs['%s']", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact),
	}
}

func (r *ResolvedArtifactRef) getReplaceTargetfromArrayIndex(idx int) []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.outputs.%s[%d]", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact, idx),
		fmt.Sprintf("%s.%s.%s.outputs[%q][%d]", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact, idx),
		fmt.Sprintf("%s.%s.%s.outputs['%s'][%d]", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact, idx),
	}
}

func (r *ResolvedArtifactRef) getReplaceTargetfromObjectKey(key string) []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.outputs.%s.%s", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact, key),
		fmt.Sprintf("%s.%s.%s.outputs[%q][%s]", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact, key),
		fmt.Sprintf("%s.%s.%s.outputs['%s'][%s]", v1beta1.ArtifactTaskPart, r.ArtifactReference.PipelineTask, v1beta1.ArtifactArtifactPart, r.ArtifactReference.Artifact, key),
	}
}
