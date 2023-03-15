/*
Copyright 2020 The Tekton Authors

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
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*MyStep)(nil)

// ConvertTo implements apis.Convertible
func (s *MyStep) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *MyStep:
		sink.ObjectMeta = s.ObjectMeta
		return s.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ss *Step) ConvertTo(ctx context.Context, sink *Step) error {
	sink.Name = ss.Name
	sink.Image = ss.Image
	sink.Command = ss.Command
	sink.Args = ss.Args
	sink.WorkingDir = ss.WorkingDir
	sink.DeprecatedPorts = ss.DeprecatedPorts
	sink.EnvFrom = ss.EnvFrom
	sink.Env = ss.Env
	sink.Resources = ss.Resources
	sink.VolumeMounts = ss.VolumeMounts
	sink.VolumeDevices = ss.VolumeDevices
	sink.DeprecatedLivenessProbe = ss.DeprecatedLivenessProbe
	sink.DeprecatedReadinessProbe = ss.DeprecatedReadinessProbe
	sink.DeprecatedLifecycle = ss.DeprecatedLifecycle
	sink.DeprecatedTerminationMessagePath = ss.DeprecatedTerminationMessagePath
	sink.ImagePullPolicy = ss.ImagePullPolicy
	sink.SecurityContext = ss.SecurityContext
	sink.DeprecatedStdin = ss.DeprecatedStdin
	sink.DeprecatedStdinOnce = ss.DeprecatedStdinOnce
	sink.DeprecatedTTY = ss.DeprecatedTTY
	sink.Script = ss.Script
	sink.Timeout = ss.Timeout
	sink.IsolatedWorkspaces = ss.IsolatedWorkspaces
	sink.OnError = ss.OnError
	sink.StdoutConfig = ss.StdoutConfig
	sink.StderrConfig = ss.StderrConfig
	sink.StepRef = ss.StepRef
	sink.Params = ss.Params
	sink.Workspaces = ss.Workspaces
	sink.Results = ss.Results
	return nil
}

// ConvertFrom implements apis.Convertible
func (s *MyStep) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *MyStep:
		s.ObjectMeta = source.ObjectMeta
		return s.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", s)
	}
}

// ConvertFrom implements apis.Convertible
func (ss *Step) ConvertFrom(ctx context.Context, source *Step) error {
	ss.Name = source.Name
	ss.Image = source.Image
	ss.Command = source.Command
	ss.Args = source.Args
	ss.WorkingDir = source.WorkingDir
	ss.DeprecatedPorts = source.DeprecatedPorts
	ss.EnvFrom = source.EnvFrom
	ss.Env = source.Env
	ss.Resources = source.Resources
	ss.VolumeMounts = source.VolumeMounts
	ss.VolumeDevices = source.VolumeDevices
	ss.DeprecatedLivenessProbe = source.DeprecatedLivenessProbe
	ss.DeprecatedReadinessProbe = source.DeprecatedReadinessProbe
	ss.DeprecatedLifecycle = source.DeprecatedLifecycle
	ss.DeprecatedTerminationMessagePath = source.DeprecatedTerminationMessagePath
	ss.ImagePullPolicy = source.ImagePullPolicy
	ss.SecurityContext = source.SecurityContext
	ss.DeprecatedStdin = source.DeprecatedStdin
	ss.DeprecatedStdinOnce = source.DeprecatedStdinOnce
	ss.DeprecatedTTY = source.DeprecatedTTY
	ss.Script = source.Script
	ss.Timeout = source.Timeout
	ss.IsolatedWorkspaces = source.IsolatedWorkspaces
	ss.OnError = source.OnError
	ss.StdoutConfig = source.StdoutConfig
	ss.StderrConfig = source.StderrConfig
	ss.StepRef = source.StepRef
	ss.Params = source.Params
	ss.Workspaces = source.Workspaces
	ss.Results = source.Results
	return nil
}
