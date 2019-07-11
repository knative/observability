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
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
)

const nullConfig = `
[OUTPUT]
    Name null
    Match *
`

const httpOutputConfig = `
[OUTPUT]
    Name http
    Match %s
    Format json
    Host %s
    Port %s
    URI %s
%s
`

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

func (sc *Config) String() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if len(sc.sinks)+len(sc.clusterSinks) == 0 {
		return nullConfig
	}
	return sc.syslogConfig() + sc.webhookConfig()
}

func (sc *Config) webhookConfig() string {
	var config string
	for _, s := range sc.sinks {
		if s.Spec.Type != "webhook" {
			continue
		}

		config += buildHTTPConfig(s.Namespace, s.Spec, false)
	}

	for _, s := range sc.clusterSinks {
		if s.Spec.Type != "webhook" {
			continue
		}

		config += buildHTTPConfig("", s.Spec, true)
	}

	return config
}

func (sc *Config) syslogConfig() string {
	sinks := make(sinkList, 0, len(sc.sinks))
	for _, s := range sc.sinks {
		if s.Spec.Type != "syslog" {
			continue
		}

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
			Name:      s.Name,
		})
	}
	sort.Slice(sinks, func(i, j int) bool {
		if sinks[i].Namespace != sinks[j].Namespace {
			return sinks[i].Namespace < sinks[j].Namespace
		}
		return sinks[i].Name < sinks[j].Name
	})

	clusterSinks := make(sinkList, 0, len(sc.clusterSinks))
	for _, s := range sc.clusterSinks {
		if s.Spec.Type != "syslog" {
			continue
		}

		var tlsConfig *tls
		if s.Spec.EnableTLS {
			tlsConfig = &tls{
				InsecureSkipVerify: s.Spec.InsecureSkipVerify,
			}
		}
		clusterSinks = append(clusterSinks, sink{
			Addr: fmt.Sprintf("%s:%d", s.Spec.Host, s.Spec.Port),
			TLS:  tlsConfig,
			Name: s.Name,
		})
	}
	sort.Slice(clusterSinks, func(i, j int) bool {
		return clusterSinks[i].Name < clusterSinks[j].Name
	})

	if len(sinks)+len(clusterSinks) == 0 {
		return ""
	}

	return sinks.String() + clusterSinks.String()
}

type sink struct {
	Addr      string `json:"addr"`
	Namespace string `json:"namespace,omitempty"`
	TLS       *tls   `json:"tls,omitempty"`
	Name      string `json:"name,omitempty"`
}

type sinkList []sink

func (ss sinkList) String() string {
	var result []string

	for _, s := range ss {
		result = append(result, s.String())
	}

	return strings.Join(result, "")
}

func (s *sink) String() string {
	var clusterOrNamespace string
	if s.Namespace != "" {
		clusterOrNamespace = fmt.Sprintf("Namespace %s", s.Namespace)
	} else {
		clusterOrNamespace = "Cluster true"
	}

	return fmt.Sprintf(`
[OUTPUT]
    Name syslog
    Match *
    InstanceName %s
    Addr %s
    %s%s
`, s.Name, s.Addr, clusterOrNamespace, s.TLS.String())

}

type tls struct {
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`
}

func (t *tls) String() string {
	if t == nil {
		return ""
	}

	b, err := json.Marshal(t)
	if err != nil {
		log.Print("unable to marshal sink TLS config")
		return ""
	}

	return fmt.Sprintf("\n    TLSConfig %s", b)
}

func buildHTTPConfig(namespace string, spec v1alpha1.SinkSpec, isCluster bool) string {
	url, err := url.Parse(spec.URL)
	if err != nil {
		return ""
	}

	var port string
	if url.Port() != "" {
		port = url.Port()
	}

	if port == "" && url.Scheme == "https" {
		port = "443"
	}

	if port == "" && url.Scheme == "http" {
		port = "80"
	}

	var extras string
	if url.Scheme == "https" {
		extras = "    tls On\n"

		if spec.InsecureSkipVerify {
			extras += "    tls.verify Off\n"
		}
	}

	match := fmt.Sprintf("*_%s_*", namespace)
	if isCluster {
		match = "*"
	}

	path := url.Path
	if path == "" {
		path = "/"
	}

	return fmt.Sprintf(
		httpOutputConfig,
		match,
		url.Hostname(),
		port,
		path,
		extras,
	)
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
