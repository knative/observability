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

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterLogSinkController(t *testing.T) {
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
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
`,
			},
		},
		{
			"Add a single TLS sink with no skip verify",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345, EnableTLS: true}},
			},
			[]string{
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
    TLSConfig {}
`,
			},
		},
		{
			"Add a single TLS sink with insecure skip verify set",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", SyslogSpec: v1alpha1.SyslogSpec{Host: "example.com", Port: 12345, EnableTLS: true}, InsecureSkipVerify: true},
			},
			[]string{
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
    TLSConfig {"insecure_skip_verify":true}
`,
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
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
`,
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true

[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-test.com
    Addr test.com:4567
    Cluster true
`,
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
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
`,
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:4567
    Cluster true
`,
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
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
`,
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12346
    Cluster true
`,
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
				`
[OUTPUT]
    Name syslog
    Match *
    InstanceName sink-example.com
    Addr example.com:12345
    Cluster true
`,
				`
[OUTPUT]
    Name null
    Match *
`,
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

	t.Run("it does not update log sinks when non-spec properties have changed", func(t *testing.T) {
		type SinkChangeTest struct {
			name string
			os   *v1alpha1.ClusterLogSink
			ns   *v1alpha1.ClusterLogSink
		}

		specs := []SinkChangeTest{
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
					sink.NewConfig(),
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
	})

	t.Run("it should not update when there are no changes between cluster log sinks", func(t *testing.T) {
		spyPatcher := &spyConfigMapPatcher{}
		spyDeleter := &spyDaemonSetPodDeleter{}
		c := sink.NewClusterController(spyPatcher, spyDeleter, sink.NewConfig())

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
	})

	t.Run("it should not panic if it receives a non cluster log sink type", func(t *testing.T) {
		c := sink.NewClusterController(
			&spyConfigMapPatcher{},
			&spyDaemonSetPodDeleter{},
			sink.NewConfig(),
		)
		//shouldn't panic
		c.OnAdd("")
		c.OnDelete(1)
		c.OnUpdate(nil, nil)
	})
}
