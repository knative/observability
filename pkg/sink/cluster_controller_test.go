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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
)

var _ = Describe("ClusterController", func() {
	DescribeTable("Sink Add, Update, and Delete",
		func(operations []string, specs []v1alpha1.SinkSpec, patches []string) {
			spyConfigMapPatcher := &spyConfigMapPatcher{}
			spyDaemonSetPodDeleter := &spyDaemonSetPodDeleter{}

			c := sink.NewClusterController(spyConfigMapPatcher, spyDaemonSetPodDeleter, sink.NewConfig())
			for i, spec := range specs {
				d := &v1alpha1.ClusterLogSink{
					Spec: spec,
				}
				switch operations[i] {
				case "add":
					c.OnAdd(d)
				case "delete":
					c.OnDelete(d)
				case "update":
					c.OnUpdate(nil, d)
				}
			}
			spyConfigMapPatcher.expectPatches(patches)
			Expect(spyDaemonSetPodDeleter.Selector).To(Equal("app=fluent-bit-ds"))
		},
		Entry("Add a single sink",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
			},
		),
		Entry("Add a single TLS sink with no skip verify",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345, EnableTLS: true},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"tls\":{}}]\n",
			},
		),
		Entry("Add a single TLS sink with insecure skip verify set",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345, EnableTLS: true, InsecureSkipVerify: true},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\",\"tls\":{\"insecure_skip_verify\":true}}]\n",
			},
		),
		Entry("Add multiple sinks",
			[]string{"add", "add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "test.com", Port: 4567},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"test.com:4567\"}]\n",
			},
		),
		Entry("Delete sink",
			[]string{"add", "delete"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "example.com", Port: 12345},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
				"\n[OUTPUT]\n    Name null\n    Match *\n",
			},
		),
		Entry("Update sink",
			[]string{"add", "update"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "example.com", Port: 12346},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12345\"}]\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks []\n    ClusterSinks [{\"addr\":\"example.com:12346\"}]\n",
			},
		),
	)

	It("doesn't update when there are no changes to the sink", func() {
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

		Expect(spyPatcher.patchCalled).To(BeFalse())
		Expect(spyDeleter.deleteCollectionCalled).To(BeFalse())
	})

	It("doesn't panic when given something that isn't a sink", func() {
		c := sink.NewClusterController(
			&spyConfigMapPatcher{},
			&spyDaemonSetPodDeleter{},
			sink.NewConfig(),
		)

		Expect(func() {
			c.OnAdd("")
			c.OnDelete(1)
			c.OnUpdate(nil, nil)
		}).ToNot(Panic())
	})
})
