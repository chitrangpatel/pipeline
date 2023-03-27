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
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/container"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

// ResolvedTask contains the data that is needed to execute
// the TaskRun.
type ResolvedTask struct {
	TaskName string
	Kind     v1beta1.TaskKind
	TaskSpec *v1beta1.TaskSpec
}

// GetTask is a function used to retrieve Tasks.
type GetTask func(context.Context, string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error)

// GetTaskRun is a function used to retrieve TaskRuns
type GetTaskRun func(string) (*v1beta1.TaskRun, error)

// GetTaskData will retrieve the Task metadata and Spec associated with the
// provided TaskRun. This can come from a reference Task or from the TaskRun's
// metadata and embedded TaskSpec.
func GetTaskData(ctx context.Context, taskRun *v1beta1.TaskRun, getTask GetTask, k8s kubernetes.Interface, tekton clientset.Interface,
	requester remoteresource.Requester, verificationpolicies []*v1alpha1.VerificationPolicy) (*resolutionutil.ResolvedObjectMeta, *v1beta1.TaskSpec, error) {
	taskMeta := metav1.ObjectMeta{}
	var configSource *v1beta1.ConfigSource
	taskSpec := v1beta1.TaskSpec{}
	switch {
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "":
		// Get related task for taskRun
		t, source, err := getTask(ctx, taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing tasks for taskRun %s: %w", taskRun.Name, err)
		}
		taskMeta = t.TaskMetadata()
		taskSpec = t.TaskSpec()
		configSource = source
	case taskRun.Spec.TaskSpec != nil:
		taskMeta = taskRun.ObjectMeta
		taskSpec = *taskRun.Spec.TaskSpec
		// TODO: if we want to set source for embedded taskspec, set it here.
		// https://github.com/tektoncd/pipeline/issues/5522
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Resolver != "":
		task, source, err := getTask(ctx, taskRun.Name)
		switch {
		case err != nil:
			return nil, nil, err
		case task == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no task was returned")
		default:
			taskMeta = task.TaskMetadata()
			taskSpec = task.TaskSpec()
		}
		configSource = source
	default:
		return nil, nil, fmt.Errorf("taskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}

	// identify artifacts
	err, preSteps, postSteps := expandArtifacts(ctx, k8s, tekton, requester, &taskSpec, taskRun, verificationpolicies)
	if err != nil {
		return nil, nil, err
	}

	steps := []v1beta1.Step{}
	stepsIncluded := sets.NewString()
	for _, s := range preSteps {
		if !stepsIncluded.Has(s.Name) {
			stepsIncluded.Insert(s.Name)
			steps = append(steps, s)
		}
	}
	for _, s := range taskSpec.Steps {
		if !stepsIncluded.Has(s.Name) {
			stepsIncluded.Insert(s.Name)
			steps = append(steps, s)
		}
	}
	for _, s := range postSteps {
		if !stepsIncluded.Has(s.Name) {
			stepsIncluded.Insert(s.Name)
			steps = append(steps, s)
		}
	}
	taskSpec.Steps = steps
	taskSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta:   &taskMeta,
		ConfigSource: configSource,
	}, &taskSpec, nil
}

func expandArtifacts(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface,
	requester remoteresource.Requester, taskspec *v1beta1.TaskSpec, taskRun *v1beta1.TaskRun, verificationpolicies []*v1alpha1.VerificationPolicy) (error, []v1beta1.Step, []v1beta1.Step) {
	cfg := config.FromContextOrDefaults(ctx)
	artifactsInConfig := map[string]*v1beta1.TaskRef{}
	if cfg.ArtifactConfig != nil {
		for _, art := range cfg.ArtifactConfig.Type {
			artifactsInConfig[art.Type] = &v1beta1.TaskRef{
				Name: art.TaskRef,
			}
		}
	}

	preStepsToFetch := []v1beta1.Step{}
	postStepsToFetch := []v1beta1.Step{}
	if taskRun.Spec.Artifacts != nil {
		for _, ia := range taskRun.Spec.Artifacts.Inputs {
			if ia.TaskRef == nil {
				if _, ok := artifactsInConfig[ia.Type]; ok {
					ia.TaskRef = artifactsInConfig[ia.Type]
				}
			}
			if ia.TaskRef != nil {
				getTask := GetVerifiedTaskFunc(ctx, k8s, tekton, requester, taskRun, ia.TaskRef, taskRun.Name, taskRun.Namespace, taskRun.Spec.ServiceAccountName, verificationpolicies)
				task, _, err := getTask(ctx, ia.TaskRef.Name)
				if err != nil {
					return err, preStepsToFetch, postStepsToFetch
				}
				// Apply replacements from mapped to actual artifact name here.
				stringReplacements := map[string]string{}
				arrayReplacements := map[string][]string{}
				if task.TaskSpec().Artifacts != nil {
					for _, tia := range task.TaskSpec().Artifacts.Inputs {
						if tia.Name != ia.Name {
							stringReplacements[fmt.Sprintf("artifacts.inputs.%s.uri", tia.Name)] = fmt.Sprintf("$(artifacts.inputs.%s.uri)", ia.Name)
							stringReplacements[fmt.Sprintf("artifacts.inputs.%s.digest", tia.Name)] = fmt.Sprintf("$(artifacts.inputs.%s.digest)", ia.Name)
							stringReplacements[fmt.Sprintf("artifacts.inputs.%s.location", tia.Name)] = fmt.Sprintf("$(artifacts.inputs.%s.location)", ia.Name)
							stringReplacements[fmt.Sprintf("artifacts.inputs.%s.path", tia.Name)] = fmt.Sprintf("$(artifacts.inputs.%s.path)", ia.Name)
						}
					}
				}
				steps := task.TaskSpec().Steps
				// Apply variable expansion to steps fields.
				for i := range steps {
					container.ApplyStepReplacements(&steps[i], stringReplacements, arrayReplacements)
					preStepsToFetch = append(preStepsToFetch, steps[i])
				}
			}
		}
		for _, oa := range taskRun.Spec.Artifacts.Outputs {
			if oa.TaskRef == nil {
				if _, ok := artifactsInConfig[oa.Type]; ok {
					oa.TaskRef = artifactsInConfig[oa.Type]
				}
			}
			if oa.TaskRef != nil {
				getTask := GetVerifiedTaskFunc(ctx, k8s, tekton, requester, taskRun, oa.TaskRef, taskRun.Name, taskRun.Namespace, taskRun.Spec.ServiceAccountName, verificationpolicies)
				task, _, err := getTask(ctx, oa.TaskRef.Name)
				if err != nil {
					return err, preStepsToFetch, postStepsToFetch
				}
				// Apply replacements from mapped to actual artifact name here.
				stringReplacements := map[string]string{}
				arrayReplacements := map[string][]string{}
				if task.TaskSpec().Artifacts != nil {
					for _, toa := range task.TaskSpec().Artifacts.Outputs {
						if toa.Name != oa.Name {
							stringReplacements[fmt.Sprintf("artifacts.outputs.%s.uri", toa.Name)] = fmt.Sprintf("$(artifacts.outputs.%s.uri)", oa.Name)
							stringReplacements[fmt.Sprintf("artifacts.outputs.%s.digest", toa.Name)] = fmt.Sprintf("$(artifacts.outputs.%s.digest)", oa.Name)
							stringReplacements[fmt.Sprintf("artifacts.outputs.%s.location", toa.Name)] = fmt.Sprintf("$(artifacts.outputs.%s.location)", oa.Name)
							stringReplacements[fmt.Sprintf("artifacts.outputs.%s.path", toa.Name)] = fmt.Sprintf("$(artifacts.outputs.%s.path)", oa.Name)
						}
					}
				}
				steps := task.TaskSpec().Steps
				// Apply variable expansion to steps fields.
				for i := range steps {
					container.ApplyStepReplacements(&steps[i], stringReplacements, arrayReplacements)
					postStepsToFetch = append(postStepsToFetch, steps[i])
				}
			}
		}
	}
	return nil, preStepsToFetch, postStepsToFetch
}
