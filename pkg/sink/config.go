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
package sink

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
)

// TODO: make sure the omitempty on namespace doesn't break tests
type sink struct {
	Addr      string `json:"addr"`
	Namespace string `json:"namespace,omitempty"`
	TLS       *tls   `json:"tls,omitempty"`
	name      string
}

type tls struct {
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`
}

const nullConfig = "\n[OUTPUT]\n    Name null\n    Match *\n"

type Config struct {
	mu           sync.Mutex
	sinks        map[string]*v1alpha1.LogSink
	clusterSinks map[string]*v1alpha1.ClusterLogSink
}

func NewConfig() *Config {
	return &Config{
		sinks:        make(map[string]*v1alpha1.LogSink),
		clusterSinks: make(map[string]*v1alpha1.ClusterLogSink),
	}
}

// TODO: Refactor
func (sc *Config) String() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if len(sc.sinks)+len(sc.clusterSinks) == 0 {
		return nullConfig
	}
	sinks := make([]sink, 0, len(sc.sinks))
	for _, s := range sc.sinks {
		var tlsConfig *tls
		if s.Spec.EnableTLS {
			tlsConfig = &tls{
				InsecureSkipVerify: s.Spec.InsecureSkipVerify,
			}
		}
		sinks = append(sinks, sink{
			Addr:      fmt.Sprintf("%s:%d", s.Spec.Host, s.Spec.Port),
			Namespace: canonicalNamespace(s.Namespace),
			TLS:       tlsConfig,
			name:      s.Name,
		})
	}
	sort.Slice(sinks, func(i, j int) bool {
		if sinks[i].Namespace != sinks[j].Namespace {
			return sinks[i].Namespace < sinks[j].Namespace
		}
		return sinks[i].name < sinks[j].name
	})
	// TODO: don't return null config yet. just set to empty json
	sinksJSON, err := json.Marshal(sinks)
	if err != nil {
		log.Print("unable to marshal sinks")
		sinksJSON = []byte("[]")
	}

	clusterSinks := make([]sink, 0, len(sc.clusterSinks))
	for _, s := range sc.clusterSinks {
		var tlsConfig *tls
		if s.Spec.EnableTLS {
			tlsConfig = &tls{
				InsecureSkipVerify: s.Spec.InsecureSkipVerify,
			}
		}
		clusterSinks = append(clusterSinks, sink{
			Addr: fmt.Sprintf("%s:%d", s.Spec.Host, s.Spec.Port),
			TLS:  tlsConfig,
			name: s.Name,
		})
	}
	sort.Slice(clusterSinks, func(i, j int) bool {
		return clusterSinks[i].name < clusterSinks[j].name
	})
	clusterSinksJSON, err := json.Marshal(clusterSinks)
	if err != nil {
		log.Print("unable to marshal cluster sinks")
		clusterSinksJSON = []byte("[]")
	}

	return fmt.Sprintf(`
[OUTPUT]
    Name syslog
    Match *
    Sinks %s
    ClusterSinks %s
`, sinksJSON, clusterSinksJSON)
}

func (sc *Config) UpsertSink(s *v1alpha1.LogSink) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.sinks[key(s)] = s
}

func (sc *Config) UpsertClusterSink(cs *v1alpha1.ClusterLogSink) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.clusterSinks[clusterKey(cs)] = cs
}

func (sc *Config) DeleteSink(s *v1alpha1.LogSink) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.sinks, key(s))
}

func (sc *Config) DeleteClusterSink(s *v1alpha1.ClusterLogSink) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.clusterSinks, clusterKey(s))
}

func canonicalNamespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

func key(s *v1alpha1.LogSink) string {
	return fmt.Sprintf("%s|%s", s.Namespace, s.Name)
}

func clusterKey(s *v1alpha1.ClusterLogSink) string {
	return fmt.Sprintf("%s|%s", s.ClusterName, s.Name)
}
