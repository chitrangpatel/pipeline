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
	"log"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/container"
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

func expandStep(ctx context.Context, s v1beta1.Step, k8s kubernetes.Interface, tekton clientset.Interface,
	requester remoteresource.Requester, taskspec *v1beta1.TaskSpec, taskrun *v1beta1.TaskRun, verificationpolicies []*v1alpha1.VerificationPolicy) (error, []v1beta1.Step) {
	log.Println("EXPANDING STEPSS...")
	log.Println(s.Name)
	if s.Uses != nil {
		var defaults []v1beta1.ParamSpec
		if len(taskspec.Params) > 0 {
			defaults = append(defaults, taskspec.Params...)
		}
		stringReplacements, arrayReplacements := ApplyParameterReplacements(ctx, taskspec, taskrun, defaults...)

		// Set and overwrite params with the ones from Uses
		uStrings, uArrays := paramsFromUses(ctx, s.Uses)
		for k, v := range uStrings {
			stringReplacements[k] = v
		}
		for k, v := range uArrays {
			arrayReplacements[k] = v
		}

		// workspace replacements
		for _, ws := range s.Uses.Workspaces {
			for _, attr := range []string{"path", "bound", "volume", "claim"} {
				stringReplacements[fmt.Sprintf("workspaces.%s.%s", ws.Name, attr)] = fmt.Sprintf("$(workspaces.%s.%s)", ws.Workspace, attr)
			}
		}

		stps := []v1beta1.Step{}
		log.Println("FOUND USES IN STEP...", s.Name, s.Uses.TaskRef)
		// Fetch taskRef
		// getTask := GetVerifiedTaskFunc(ctx, k8s, tekton, requester, taskrun, s.Uses.TaskRef, s.Name, taskrun.Namespace, taskrun.Spec.ServiceAccountName, verificationpolicies)
		getTask := GetVerifiedTaskFunc(ctx, k8s, tekton, requester, taskrun, s.Uses.TaskRef, s.Name, taskrun.Namespace, taskrun.Spec.ServiceAccountName, verificationpolicies)
		task, _, err := getTask(ctx, s.Name)
		if err != nil {
			return err, stps
		}
		log.Println(task.TaskMetadata().Name)
		spec := task.TaskSpec()
		/*err, expandedSteps := expandStep(ctx, step, k8s, tekton, requester, &spec, taskrun, verificationpolicies)
		if err != nil {
			return err, stps
		}*/
		defaults = []v1beta1.ParamSpec{}
		if len(spec.Params) > 0 {
			defaults = append(defaults, spec.Params...)
		}
		sr, ar := ApplyParameterReplacements(ctx, &spec, taskrun, defaults...)
		for k, v := range sr {
			stringReplacements[k] = v
		}
		for k, v := range ar {
			arrayReplacements[k] = v
		}

		// Apply task result substitution
		patterns := []string{
			"results.%s.path",
			"results[%q].path",
			"results['%s'].path",
		}

		for _, result := range spec.Results {
			for _, pattern := range patterns {
				stringReplacements[fmt.Sprintf(pattern, result.Name)] = filepath.Join(pipeline.DefaultResultPath, result.Name)
			}
		}

		// Apply variable expansion to steps fields.
		// for i := range expandedSteps {
		for i := range task.TaskSpec().Steps {
			// for i := range spec.Steps {
			// container.ApplyStepReplacements(&expandedSteps[i], stringReplacements, arrayReplacements)
			container.ApplyStepReplacements(&task.TaskSpec().Steps[i], stringReplacements, arrayReplacements)
		}

		// for _, es := range expandedSteps {
		for _, es := range task.TaskSpec().Steps {
			found := false
			for _, st := range stps {
				if es.Name == st.Name {
					found = true
					break
				}
			}
			if !found {
				es.Name = fmt.Sprintf("%s-%s", s.Name, es.Name)
				stps = append(stps, es)
			}
		}
		return nil, stps
	} else {
		return nil, []v1beta1.Step{s}
	}
}

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
		err, es := expandStep(ctx, step, k8s, tekton, requester, &taskSpec, taskRun, verificationpolicies)
		if err != nil {
			return nil, nil, err
		}
		steps = append(steps, es...)
	}
	taskSpec.Steps = steps
	taskSpec.SetDefaults(ctx)
	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta:   &taskMeta,
		ConfigSource: configSource,
	}, &taskSpec, nil
}
