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
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ArtifactRef is a type that represents a reference to a task run artifact
type ArtifactRef struct {
	PipelineTask   string `json:"pipelineTask"`
	Artifact       string `json:"artifact"`
	ArtifactsIndex int    `json:"artifactsIndex"`
	Property       string `json:"property"`
}

const (
	artifactExpressionFormat = "tasks.<taskName>.artifacts.<artifactName>"
	// Artifact expressions of the form <artifactName>.<attribute> will be treated as object artifacts.
	// If a string artifact name contains a dot, brackets should be used to differentiate it from an object artifact.
	// https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-artifact-types.md#collisions-with-builtin-variable-replacement
	objectArtifactExpressionFormat = "tasks.<taskName>.artifacts.<objectArtifactName>.<individualAttribute>"
	// ArtifactTaskPart Constant used to define the "tasks" part of a pipeline artifact reference
	ArtifactTaskPart = "tasks"
	// ArtifactFinallyPart Constant used to define the "finally" part of a pipeline artifact reference
	ArtifactFinallyPart = "finally"
	// ArtifactArtifactPart Constant used to define the "artifacts" part of a pipeline artifact reference
	ArtifactArtifactPart = "artifacts"
	// ArtifactNameFormat Constant used to define the regex Artifact.Name should follow
	ArtifactNameFormat = `^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`
)

var artifactNameFormatRegex = regexp.MustCompile(ArtifactNameFormat)

func parseArtifactExpression(substitutionExpression string) (string, string, int, string, error) {
	if looksLikeArtifactRef(substitutionExpression) {
		subExpressions := strings.Split(substitutionExpression, ".")
		// For string result: tasks.<taskName>.artifacts.inputs.<stringResultName>
		// For array result: tasks.<taskName>.artifacts.inputs.<arrayResultName>[index]
		if len(subExpressions) == 5 {
			artifactName, stringIdx := ParseArtifactName(subExpressions[4])
			if stringIdx != "" {
				intIdx, _ := strconv.Atoi(stringIdx)
				return subExpressions[1], artifactName, intIdx, "", nil
			}
			return subExpressions[1], artifactName, 0, "", nil
		} else if len(subExpressions) == 6 {
			// For object type result: tasks.<taskName>.artifacts.inputs.<objectResultName>.<individualAttribute>
			return subExpressions[1], subExpressions[4], 0, subExpressions[5], nil
		}
	}
	return "", "", 0, "", fmt.Errorf("must be one of the form 1). %q; 2). %q", resultExpressionFormat, objectResultExpressionFormat)
}

func NewArtifactRefs(expressions []string) []*ArtifactRef {
	var artifactRefs []*ArtifactRef
	for _, expression := range expressions {
		pipelineTask, artifact, index, property, err := parseArtifactExpression(expression)
		// If the expression isn't a artifact but is some other expression,
		// parseArtifactExpression will return an error, in which case we just skip that expression,
		// since although it's not a artifact ref, it might be some other kind of reference
		if err == nil {
			artifactRefs = append(artifactRefs, &ArtifactRef{
				PipelineTask:   pipelineTask,
				Artifact:       artifact,
				ArtifactsIndex: index,
				Property:       property,
			})
		}
	}
	return artifactRefs
}

// GetVarSubstitutionExpressionsForPipelineArtifact extracts all the value between "$(" and ")"" for a pipeline artifact
func GetVarSubstitutionExpressionsForPipelineArtifact(artifact Artifact) ([]string, bool) {
	allExpressions := validateString(artifact.Value.StringVal)
	for _, v := range artifact.Value.ArrayVal {
		allExpressions = append(allExpressions, validateString(v)...)
	}
	for _, v := range artifact.Value.ObjectVal {
		allExpressions = append(allExpressions, validateString(v)...)
	}
	return allExpressions, len(allExpressions) != 0
}

// ParseArtifactName parse the input string to extract artifactName and artifact index.
// Array indexing:
// Input:  anArrayArtifact[1]
// Output: anArrayArtifact, "1"
// Array star reference:
// Input:  anArrayArtifact[*]
// Output: anArrayArtifact, "*"
func ParseArtifactName(artifactName string) (string, string) {
	stringIdx := strings.TrimSuffix(strings.TrimPrefix(arrayIndexingRegex.FindString(artifactName), "["), "]")
	artifactName = arrayIndexingRegex.ReplaceAllString(artifactName, "")
	return artifactName, stringIdx
}

// LooksLikeContainsArtifactRefs attempts to check if param or a pipeline artifact looks like it contains any
// artifact references.
// This is useful if we want to make sure the param looks like a ArtifactReference before
// performing strict validation
func LooksLikeContainsArtifactRefs(expressions []string) bool {
	for _, expression := range expressions {
		if looksLikeArtifactRef(expression) {
			return true
		}
	}
	return false
}

// looksLikeArtifactRef attempts to check if the given string looks like it contains any
// artifact references. Returns true if it does, false otherwise
func looksLikeArtifactRef(expression string) bool {
	subExpressions := strings.Split(expression, ".")
	return len(subExpressions) >= 4 && (subExpressions[0] == ArtifactTaskPart || subExpressions[0] == ArtifactFinallyPart) && subExpressions[2] == ArtifactArtifactPart
}

func GetVarSubstitutionExpressionsForInputArtifact(artifact Artifact) ([]string, bool) {
	var allExpressions []string
	switch artifact.Value.Type {
	case ParamTypeString:
		// string type
		allExpressions = append(allExpressions, validateString(artifact.Value.StringVal)...)
	case ParamTypeObject:
		// object type
		for _, value := range artifact.Value.ObjectVal {
			allExpressions = append(allExpressions, validateString(value)...)
		}
	default:
		return nil, false
	}
	return allExpressions, len(allExpressions) != 0
}

// PipelineTaskArtifactRefs walks all the places a artifact reference can be used
// in a PipelineTask and returns a list of any references that are found.
func PipelineTaskArtifactRefs(pt *PipelineTask) []*ArtifactRef {
	refs := []*ArtifactRef{}
	for _, p := range pt.extractAllParams() {
		expressions, _ := GetVarSubstitutionExpressionsForParam(p)
		refs = append(refs, NewArtifactRefs(expressions)...)
	}
	for _, whenExpression := range pt.WhenExpressions {
		expressions, _ := whenExpression.GetVarSubstitutionExpressions()
		refs = append(refs, NewArtifactRefs(expressions)...)
	}
	if pt.Artifacts != nil {
		for _, ia := range pt.Artifacts.Inputs {
			expressions, _ := GetVarSubstitutionExpressionsForInputArtifact(ia)
			refs = append(refs, NewArtifactRefs(expressions)...)
		}
	}
	return refs
}
