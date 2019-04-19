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
	"fmt"
	"testing"
	"time"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"name\":\"sink-example.com\"}]\n",
			},
		},
		{
			"Add a single TLS sink with no skip verify",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345, EnableTLS: true}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"tls\":{},\"name\":\"sink-example.com\"}]\n",
			},
		},
		{
			"Add a single TLS sink with insecure skip verify set",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345, EnableTLS: true, InsecureSkipVerify: true}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"tls\":{\"insecure_skip_verify\":true},\"name\":\"sink-example.com\"}]\n",
			},
		},
		{
			"Add sink multiple times",
			[]string{"add", "add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345}},
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "test.com", Port: 4567}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"name\":\"sink-example.com\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"name\":\"sink-example.com\"},{\"addr\":\"test.com:4567\",\"name\":\"sink-test.com\"}]\n",
			},
		},
		{
			"Add same name is update",
			[]string{"add", "add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345}},
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 4567}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"name\":\"sink-example.com\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:4567\",\"name\":\"sink-example.com\"}]\n",
			},
		},
		{
			"Update sink",
			[]string{"add", "update"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345}},
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12346}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"name\":\"sink-example.com\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12346\",\"name\":\"sink-example.com\"}]\n",
			},
		},
		{
			"Delete sink",
			[]string{"add", "delete"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345}},
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345}},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    StatsAddr 127.0.0.1:5000\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"name\":\"sink-example.com\"}]\n",
				"\n[OUTPUT]\n    Name null\n    Match *\n    StatsAddr 127.0.0.1:5000\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spyConfigMapPatcher := &spyConfigMapPatcher{}
			spyDaemonSetPodDeleter := &spyDaemonSetPodDeleter{}

			c := sink.NewClusterController(spyConfigMapPatcher, spyDaemonSetPodDeleter, sink.NewConfig("127.0.0.1:5000"))
			for i, spec := range test.specs {
				d := &v1alpha1.ClusterLogSink{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("sink-%s", spec.Host),
					},
					Spec: spec,
				}
				switch test.operations[i] {
				case "add":
					c.OnAdd(d)
				case "delete":
					c.OnDelete(d)
				case "update":
					old := &v1alpha1.ClusterLogSink{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("sink-%s", test.specs[i-1].Host),
						},
						Spec: test.specs[i-1],
					}
					c.OnUpdate(old, d)
				}
			}

			var expectedPatches []spyPatch
			for _, p := range test.patches {
				expectedPatches = append(expectedPatches, spyPatch{
					Path:  "/data/outputs.conf",
					Value: p,
				})
			}

			spyConfigMapPatcher.expectPatches(expectedPatches, t)
			if spyDaemonSetPodDeleter.Selector != "app=fluent-bit" {
				t.Errorf("DaemonSet PodDeleter not equal: Expected: %s, Actual: %s", spyDaemonSetPodDeleter.Selector, "app=fluent-bit")
			}
		})
	}
}

func TestClusterDoesNotUpdateWhenNonSpecPropertiesHaveChanged(t *testing.T) {
	type SinkChangeTest struct {
		name string
		os   *v1alpha1.ClusterLogSink
		ns   *v1alpha1.ClusterLogSink
	}

	specs := []SinkChangeTest{
		{
			name: "change status state",
			os: &v1alpha1.ClusterLogSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sink",
				},
				Status: v1alpha1.SinkStatus{
					State: "Running1",
				},
			},
			ns: &v1alpha1.ClusterLogSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sink",
				},
				Status: v1alpha1.SinkStatus{
					State: "Running2",
				},
			},
		},
		{
			name: "change status timestamp",
			os: &v1alpha1.ClusterLogSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sink",
				},
				Status: v1alpha1.SinkStatus{
					LastSuccessfulSend: v1.MicroTime{
						Time: time.Time{},
					},
				},
			},
			ns: &v1alpha1.ClusterLogSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sink",
				},
				Status: v1alpha1.SinkStatus{
					LastSuccessfulSend: v1.MicroTime{
						Time: time.Now(),
					},
				},
			},
		},
		{
			name: "change object labels",
			os: &v1alpha1.ClusterLogSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sink",
				},
			},
			ns: &v1alpha1.ClusterLogSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sink",
					Labels: map[string]string{
						"test": "labelval",
					},
				},
			},
		},
	}

	for _, sc := range specs {
		t.Run(sc.name, func(t *testing.T) {
			spyPatcher := &spyConfigMapPatcher{}
			spyDeleter := &spyDaemonSetPodDeleter{}
			c := sink.NewClusterController(
				spyPatcher,
				spyDeleter,
				sink.NewConfig("127.0.0.1:5000"),
			)
			c.OnUpdate(sc.os, sc.ns)
			if spyPatcher.patchCalled {
				t.Errorf("Expected patch to not be called")
			}
			if spyDeleter.deleteCollectionCalled {
				t.Errorf("Expected delete to not be called")
			}
		})
	}
}

func TestNoopChange(t *testing.T) {
	spyPatcher := &spyConfigMapPatcher{}
	spyDeleter := &spyDaemonSetPodDeleter{}
	c := sink.NewClusterController(spyPatcher, spyDeleter, sink.NewConfig("127.0.0.1:5000"))

	s1 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sink",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sink",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
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
		sink.NewConfig("127.0.0.1:5000"),
	)
	//shouldn't panic
	c.OnAdd("")
	c.OnDelete(1)
	c.OnUpdate(nil, nil)
}
