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

package config

import (
	"encoding/json"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
)

// +k8s:deepcopy-gen=true
type ArtifactType struct {
	Type    string `json:"type"`
	TaskRef string `json:"taskRef"`
}

// +k8s:deepcopy-gen=true
type ArtifactConfig struct {
	Type []*ArtifactType `json:"artifact-type"`
}

// GetFeatureFlagsConfigName returns the name of the configmap containing all
// feature flags.
func GetArtifactConfigName() string {
	if e := os.Getenv("ARTIFACT_CONFIG"); e != "" {
		return e
	}
	return "artifact-config"
}

// NewFeatureFlagsFromMap returns a Config given a map corresponding to a ConfigMap
func NewArtifactConfigFromMap(cfgMap map[string]string) (*ArtifactConfig, error) {
	tc := ArtifactConfig{}
	log.Println("NEW ARTIFACT CONFIG FROM MAP::::")
	log.Println(cfgMap)
	if err := setArtifactConfig(cfgMap, nil, &tc); err != nil {
		return nil, err
	}
	log.Println("FINAL: ", tc)
	return &tc, nil
}

func setArtifactConfig(cfgMap map[string]string, defaultValue *ArtifactConfig, artifactConfig *ArtifactConfig) error {
	value := ArtifactConfig{}
	if cfg, ok := cfgMap["type"]; ok {
		log.Println("UNMARSHALING>>>")
		log.Println(cfg)
		err := json.Unmarshal([]byte(cfg), &value)
		if err != nil {
			return err
		}
	}
	log.Println("VALUE", value)
	*artifactConfig = value
	return nil
}

// NewFeatureFlagsFromConfigMap returns a Config for the given configmap
func NewArtifactConfigFromConfigMap(config *corev1.ConfigMap) (*ArtifactConfig, error) {
	return NewArtifactConfigFromMap(config.Data)
}
