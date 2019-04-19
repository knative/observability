/*
Copyright 2018 The Knative Authors

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
package metric

import (
	"bytes"
	"log"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
)

const emptyConfig = `[inputs]

  [[inputs.cpu]]

[outputs]

  [[outputs.discard]]
`

type telegrafConfig struct {
	GlobalTags map[string]string                   `toml:"global_tags"`
	Inputs     map[string][]map[string]interface{} `toml:"inputs"`
	Outputs    map[string][]map[string]interface{} `toml:"outputs"`
}

func (t telegrafConfig) String() string {
	if len(t.Inputs) == 0 || len(t.Outputs) == 0 {
		return emptyConfig
	}

	buf := &bytes.Buffer{}
	encoder := toml.NewEncoder(buf)
	err := encoder.Encode(t)
	if err != nil {
		log.Printf("Unable to encode telegraf config: %s", err)
		return ""
	}

	return buf.String()
}

type Config struct {
	mu                        sync.RWMutex
	useInsecureKubernetesPort bool
	clusterName               string
	clusterSinks              map[string]v1alpha1.ClusterMetricSink
}

func NewConfig(useInsecureKubernetesPort bool, clusterName string) *Config {
	return &Config{
		clusterSinks:              make(map[string]v1alpha1.ClusterMetricSink),
		useInsecureKubernetesPort: useInsecureKubernetesPort,
		clusterName:               clusterName,
	}
}

func (c *Config) String() string {
	config := telegrafConfig{
		Inputs:  c.defaultInputs(),
		Outputs: make(map[string][]map[string]interface{}),
	}

	if c.clusterName != "" {
		config.GlobalTags = map[string]string{"cluster_name": c.clusterName}
	}

	func() {
		c.mu.RLock()
		defer c.mu.RUnlock()
		for _, cms := range c.clusterSinks {
			for _, input := range cms.Spec.Inputs {
				t, ok := input["type"].(string)
				if !ok {
					continue
				}

				newInputs := make(map[string]interface{}, len(input)-1)
				for k, v := range input {
					if k != "type" {
						newInputs[k] = v
					}
				}
				config.Inputs[t] = append(config.Inputs[t], newInputs)
			}
			for _, output := range cms.Spec.Outputs {
				t, ok := output["type"].(string)
				if !ok {
					continue
				}

				newOutputs := make(map[string]interface{}, len(output)-1)
				for k, v := range output {
					if k != "type" {
						newOutputs[k] = v
					}
				}
				config.Outputs[t] = append(config.Outputs[t], newOutputs)
			}
		}
	}()

	return config.String()
}

func (c *Config) UpsertSink(cms v1alpha1.ClusterMetricSink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clusterSinks[cms.ObjectMeta.Name] = cms
}

func (c *Config) DeleteSink(cms v1alpha1.ClusterMetricSink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.clusterSinks, cms.ObjectMeta.Name)
}

func (c *Config) defaultInputs() map[string][]map[string]interface{} {
	inputMap := make(map[string][]map[string]interface{})
	if c.useInsecureKubernetesPort {
		inputMap["kubernetes"] = []map[string]interface{}{
			{
				"url": "http://127.0.0.1:10255",
			},
		}
	} else {
		inputMap["kubernetes"] = []map[string]interface{}{
			{
				"bearer_token":         "/var/run/secrets/kubernetes.io/serviceaccount/token",
				"insecure_skip_verify": true,
				"url":                  "https://127.0.0.1:10250",
			},
		}
	}

	return inputMap
}
