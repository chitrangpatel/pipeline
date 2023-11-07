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

package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/pod"
)

func main() {
	var resultsDir string
	var resultNames string
	var declaredStepResults string
	var undeclaredStepResults string
	flag.StringVar(&resultsDir, "results-dir", pipeline.DefaultResultPath, "Path to the results directory. Default is /tekton/results")
	flag.StringVar(&resultNames, "result-names", "", "comma separated result names to expect from the steps running in the pod. eg. foo,bar,baz")
	flag.StringVar(&declaredStepResults, "declared-results", "", "step results explicitly requested by the Task.")
	flag.StringVar(&undeclaredStepResults, "undeclared-results", "", "possible step results that need to be auto-surfaced.")
	flag.Parse()
	if resultNames == "" {
		log.Fatal("result-names were not provided")
	}
	expectedResults := strings.Split(resultNames, ",")
	declaredResults := map[string]string{}
	err := json.Unmarshal([]byte(declaredStepResults), &declaredResults)
	if err != nil {
		log.Fatal(err)
	}
	undeclaredResults := map[string][]string{}
	err = json.Unmarshal([]byte(undeclaredStepResults), &undeclaredResults)
	if err != nil {
		log.Fatal(err)
	}
	err = sidecarlogresults.LookForResults(os.Stdout, pod.RunDir, resultsDir, expectedResults, declaredResults, undeclaredResults)
	if err != nil {
		log.Fatal(err)
	}
}
