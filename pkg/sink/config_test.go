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
package sink_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
)

var emptyConfig = `
[OUTPUT]
    Name null
    Match *
`

func TestEmptyConfig(t *testing.T) {
	config := sink.NewConfig().String()
	if config != emptyConfig {
		t.Errorf("Empty Config not equal: Expected: %s Actual: %s", emptyConfig, config)
	}
}

func TestSingleSink(t *testing.T) {
	sc := sink.NewConfig()
	sink := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name",
			Namespace: "some-namespace",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}

	sc.UpsertSink(sink)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "some-namespace",
				},
			},
			ClusterSinks: []ClusterSink{},
		},
		t,
	)
}

func TestMultipleSinks(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-2",
			Namespace: "ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 45678,
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertSink(s2)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "ns1",
				},
				{
					Addr:      "example.org:45678",
					Namespace: "ns2",
				},
			},
		},
		t,
	)
}

func TestMultipleClusterSinks(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 45678,
		},
	}

	sc.UpsertClusterSink(s1)
	sc.UpsertClusterSink(s2)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			ClusterSinks: []ClusterSink{
				{
					Addr: "example.com:12345",
				},
				{
					Addr: "example.org:45678",
				},
			},
		},
		t,
	)
}

func TestSinksWithClusterSinks(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 45678,
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "ns1",
				},
			},
			ClusterSinks: []ClusterSink{
				{
					Addr: "example.org:45678",
				},
			},
		},
		t,
	)
}

func TestAllSinksRemoved(t *testing.T) {
	sc := sink.NewConfig()
	s := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "ns.example.com",
			Port: 12345,
		},
	}
	cs := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "cl.example.org",
			Port: 45678,
		},
	}

	sc.UpsertSink(s)
	sc.UpsertClusterSink(cs)
	sc.DeleteSink(s)
	sc.DeleteClusterSink(cs)
	if sc.String() != emptyConfig {
		t.Errorf("Empty Config not equal: Expected: %s Actual: %s", emptyConfig, sc.String())
	}
}

func TestRemoveSink(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "some-namespace-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example1.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example2.com",
			Port: 12345,
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)
	sc.DeleteSink(s1)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			ClusterSinks: []ClusterSink{
				{
					Addr: "example2.com:12345",
				},
			},
		},
		t,
	)
}

func TestRemoveClusterSink(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 45678,
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)
	sc.DeleteClusterSink(s2)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "ns1",
				},
			},
		},
		t,
	)
}

func TestUpdateSink(t *testing.T) {
	sc := sink.NewConfig()
	s := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "ns.example.com",
			Port: 12345,
		},
	}
	cs := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "cl.example.org",
			Port: 45678,
		},
	}

	sc.UpsertSink(s)
	sc.UpsertClusterSink(cs)
	s.Spec.Host = "ns.sample.com"
	cs.Spec.Host = "cl.sample.org"
	sc.UpsertSink(s)
	sc.UpsertClusterSink(cs)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "ns.sample.com:12345",
					Namespace: "ns1",
				},
			},
			ClusterSinks: []ClusterSink{
				{
					Addr: "cl.sample.org:45678",
				},
			},
		},
		t,
	)
}

func TestUpdateConcurrency(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-2",
			Namespace: "ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 45678,
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sc.UpsertClusterSink(s2)
	}()
	go sc.String()
	sc.UpsertSink(s1)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("timed out waiting for upserts")
	}

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "ns1",
				},
			},
			ClusterSinks: []ClusterSink{
				{
					Addr: "example.org:45678",
				},
			},
		},
		t,
	)

	done = make(chan struct{})
	go func() {
		defer close(done)
		sc.DeleteClusterSink(s2)
	}()
	sc.DeleteSink(s1)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("timed out waiting for deletes")
	}

	if sc.String() != emptyConfig {
		t.Errorf("Empty Config not equal: Expected: %s Actual: %s", emptyConfig, sc.String())
	}
}

func TestSinkOrdering(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-3",
			Namespace: "a-ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-4",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s3 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "z-ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 45678,
		},
	}
	s4 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-2",
			Namespace: "z-ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.org",
			Port: 12345,
		},
	}

	sc.UpsertSink(s4)
	sc.UpsertSink(s3)
	sc.UpsertSink(s2)
	sc.UpsertSink(s1)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "a-ns1",
				},
				{
					Addr:      "example.com:12345",
					Namespace: "default",
				},
				{
					Addr:      "example.org:45678",
					Namespace: "z-ns2",
				},
				{
					Addr:      "example.org:12345",
					Namespace: "z-ns2",
				},
			},
		},
		t,
	)
}

func TestTlsEncoding(t *testing.T) {
	sc := sink.NewConfig()
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "some-namespace",
		},
		Spec: v1alpha1.SinkSpec{
			Type:      "syslog",
			Host:      "example.com",
			Port:      12345,
			EnableTLS: true,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type:      "syslog",
			Host:      "example.com",
			Port:      12345,
			EnableTLS: true,
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "some-namespace",
				},
			},
			ClusterSinks: []ClusterSink{
				{
					Addr: "example.com:12345",
				},
			},
		},
		t,
	)

	s1.Spec.InsecureSkipVerify = true
	s2.Spec.InsecureSkipVerify = true

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "some-namespace",
					TLS: TLSConfig{
						InsecureSkipVerify: true,
					},
				},
			},
			ClusterSinks: []ClusterSink{
				{
					Addr: "example.com:12345",
					TLS: TLSConfig{
						InsecureSkipVerify: true,
					},
				},
			},
		},
		t,
	)
}

func TestEmptyNamespace(t *testing.T) {
	sc := sink.NewConfig()
	sink := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}

	sc.UpsertSink(sink)

	expectConfig(
		sc.String(),
		ConfigComparer{
			Name:  "syslog",
			Match: "*",
			NamespaceSinks: []NamespaceSink{
				{
					Addr:      "example.com:12345",
					Namespace: "default",
				},
			},
		},
		t,
	)
}

type ConfigComparer struct {
	Name           string
	Match          string
	ClusterSinks   []ClusterSink
	NamespaceSinks []NamespaceSink
}

type ClusterSink struct {
	Addr string `json:"addr"`
	TLS  TLSConfig
}

type NamespaceSink struct {
	Addr      string `json:"addr"`
	Namespace string `json:"namespace"`
	TLS       TLSConfig
}

type TLSConfig struct {
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
}

func expectConfig(conf string, compare ConfigComparer, t *testing.T) {
	conf = strings.TrimSpace(conf)

	trimmedConf := strings.TrimPrefix(conf, "[OUTPUT]\n")
	if conf == trimmedConf {
		t.Errorf("Expected conf to have output prefix")
	}

	lines := strings.Split(trimmedConf, "\n")
	props := make(map[string]string, len(lines))
	for _, line := range lines {
		kv := strings.Split(strings.TrimSpace(line), " ")
		props[kv[0]] = kv[1]
	}

	if len(compare.NamespaceSinks) != 0 {
		actualString, ok := props["Sinks"]
		if !ok {
			t.Errorf("Expected Sinks to be present on config")
		}

		var actual []NamespaceSink
		err := json.Unmarshal([]byte(actualString), &actual)
		if err != nil {
			t.Errorf("Could not Unmarshal namespace sink: %s", err)
		}

		if diff := cmp.Diff(compare.NamespaceSinks, actual); diff != "" {
			t.Errorf("As (-want, +got) = %v", diff)
		}
	} else {
		actual, ok := props["Sinks"]
		if !ok {
			t.Errorf("Expected Sinks to be present")
		}
		if actual != "[]" {
			t.Errorf("Expected ClusterSinks to be empty")
		}
	}
	if len(compare.ClusterSinks) != 0 {
		actualString, ok := props["ClusterSinks"]
		if !ok {
			t.Errorf("Expected ClusterSinks to be present on config")
		}

		var actual []ClusterSink
		err := json.Unmarshal([]byte(actualString), &actual)
		if err != nil {
			t.Errorf("Could not Unmarshal cluster sink: %s", err)
		}

		if diff := cmp.Diff(compare.ClusterSinks, actual); diff != "" {
			t.Errorf("As (-want, +got) = %v", diff)
		}
	} else {
		actual, ok := props["ClusterSinks"]
		if !ok {
			t.Errorf("Expected ClusterSinks to be present")
		}
		if actual != "[]" {
			t.Errorf("Expected ClusterSinks to be empty")
		}
	}

	{
		actualString, ok := props["Name"]
		if !ok {
			t.Errorf("Expected name to be present on config")
		}
		if actualString != compare.Name {
			t.Errorf("Expected name to match config: Expected: %s Actual: %s", compare.Name, actualString)
		}
	}
	{
		actualString, ok := props["Match"]
		if !ok {
			t.Errorf("Expected match to be present on config")
		}
		if actualString != compare.Match {
			t.Errorf("Expected match to match config: Expected: %s Actual: %s", compare.Match, actualString)
		}
	}
}
