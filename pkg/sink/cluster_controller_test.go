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
	"testing"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
)

func TestClusterSinkModification(t *testing.T) {
	var tests = []struct {
		name       string
		operations []string
		specs      []v1alpha1.SinkSpec
		patches    []string
	}{
		{
			"Add a single sink",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
			},
		},
		{
			"Add a single TLS sink with no skip verify",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345, EnableTLS: true},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"tls\":{}}]\n",
			},
		},
		{
			"Add a single TLS sink with insecure skip verify set",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345, EnableTLS: true, InsecureSkipVerify: true},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"tls\":{\"insecure_skip_verify\":true}}]\n",
			},
		},
		{
			"Add multiple sinks",
			[]string{"add", "add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "test.com", Port: 4567},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"test.com:4567\"}]\n",
			},
		},
		{
			"Update sink",
			[]string{"add", "update"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "example.com", Port: 12346},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12346\"}]\n",
			},
		},
		{
			"Delete sink",
			[]string{"add", "delete"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "example.com", Port: 12345},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
				"\n[OUTPUT]\n    Name null\n    Match *\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spyConfigMapPatcher := &spyConfigMapPatcher{}
			spyDaemonSetPodDeleter := &spyDaemonSetPodDeleter{}

			c := sink.NewClusterController(spyConfigMapPatcher, spyDaemonSetPodDeleter, sink.NewConfig())
			for i, spec := range test.specs {
				d := &v1alpha1.ClusterLogSink{
					Spec: spec,
				}
				switch test.operations[i] {
				case "add":
					c.OnAdd(d)
				case "delete":
					c.OnDelete(d)
				case "update":
					c.OnUpdate(nil, d)
				}
			}
			spyConfigMapPatcher.expectPatches(test.patches, t)
			if spyDaemonSetPodDeleter.Selector != "app=fluent-bit-ds" {
				t.Errorf("DaemonSet PodDeleter not equal: Expected: %s, Actual: %s", spyDaemonSetPodDeleter.Selector, "app=fluent-bit-ds")
			}
		})
	}
}

func TestNoopChange(t *testing.T) {
	spyPatcher := &spyConfigMapPatcher{}
	spyDeleter := &spyDaemonSetPodDeleter{}
	c := sink.NewClusterController(spyPatcher, spyDeleter, sink.NewConfig())

	s1 := &v1alpha1.ClusterLogSink{
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: "example.com",
			Port: 12345,
		},
	}
	c.OnUpdate(s1, s2)
	if spyPatcher.patchCalled {
		t.Errorf("Expected patch to not be called")
	}
	if spyDeleter.deleteCollectionCalled {
		t.Errorf("Expected delete to not be called")
	}
}

func TestBadInputs(t *testing.T) {
	c := sink.NewClusterController(
		&spyConfigMapPatcher{},
		&spyDaemonSetPodDeleter{},
		sink.NewConfig(),
	)
	//shouldn't panic
	c.OnAdd("")
	c.OnDelete(1)
	c.OnUpdate(nil, nil)
}
