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
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		// Get related task for taskrun
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

	// TODO: Expand Steps here
	steps := []v1beta1.Step{}
	for _, step := range taskSpec.Steps {
		var s v1beta1.Step
		if step.StepRef != nil {
			name := step.Name
			localStep := &LocalStepRefResolver{
				Namespace:    taskRun.Namespace,
				Tektonclient: tekton,
			}
			referencedStep, _, err := localStep.GetStep(ctx, step.StepRef.Name)
			if err != nil {
				return nil, nil, err
			}
			s = referencedStep.MyStepSpec()
			s.Name = name
			// apply mappings
			for _, p := range step.Params {
				patterns := []string{
					"params.%s",
					"params[%q]",
					"params['%s']",
				}
				for _, pattern := range patterns {
					s.Script = strings.Replace(s.Script, fmt.Sprintf(pattern, p.Name), fmt.Sprintf(pattern, p.Value), -1)
					for _, c := range s.Command {
						c = strings.Replace(c, fmt.Sprintf(pattern, p.Name), fmt.Sprintf(pattern, p.Value), -1)
					}
					for _, a := range s.Args {
						a = strings.Replace(a, fmt.Sprintf(pattern, p.Name), fmt.Sprintf(pattern, p.Value), -1)
					}
				}
			}
			for _, r := range step.Results {
				patterns := []string{
					"results.%s",
					"results[%q]",
					"results['%s']",
				}
				for _, pattern := range patterns {
					s.Script = strings.Replace(s.Script, fmt.Sprintf(pattern, r.Name), fmt.Sprintf(pattern, r.Value), -1)
					for _, c := range s.Command {
						c = strings.Replace(c, fmt.Sprintf(pattern, r.Name), fmt.Sprintf(pattern, r.Value), -1)
					}
					for _, a := range s.Args {
						a = strings.Replace(a, fmt.Sprintf(pattern, r.Name), fmt.Sprintf(pattern, r.Value), -1)
					}
				}
			}
			for _, w := range step.Workspaces {
				pattern := "workspaces.%s"
				s.Script = strings.Replace(s.Script, fmt.Sprintf(pattern, w.Name), fmt.Sprintf(pattern, w.Value), -1)
				s.WorkingDir = strings.Replace(s.WorkingDir, fmt.Sprintf(pattern, w.Name), fmt.Sprintf(pattern, w.Value), -1)
				for _, c := range s.Command {
					c = strings.Replace(c, fmt.Sprintf(pattern, w.Name), fmt.Sprintf(pattern, w.Value), -1)
				}
				for _, a := range s.Args {
					a = strings.Replace(a, fmt.Sprintf(pattern, w.Name), fmt.Sprintf(pattern, w.Value), -1)
				}
			}
			steps = append(steps, s)
		} else {
			steps = append(steps, step)
		}
	}
	// replacing steps
	taskSpec.Steps = steps
	taskSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta:   &taskMeta,
		ConfigSource: configSource,
	}, &taskSpec, nil
}
