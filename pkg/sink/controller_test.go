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

	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
)

var _ = Describe("Controller", func() {
	DescribeTable("Sink Add, Update, and Delete",
		func(operations []string, specs []v1alpha1.SinkSpec, patches []string) {
			spyConfigMapPatcher := &spyConfigMapPatcher{}
			spyDaemonSetPodDeleter := &spyDaemonSetPodDeleter{}
			c := sink.NewController(
				spyConfigMapPatcher,
				spyDaemonSetPodDeleter,
				sink.NewConfig(),
			)
			for i, spec := range specs {
				d := &v1alpha1.LogSink{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
					},
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
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"test-ns\"}]\n    ClusterSinks []\n",
			},
		),
		Entry("Add a single TLS sink with no skip verify",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345, EnableTLS: true},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"test-ns\",\"tls\":{}}]\n    ClusterSinks []\n",
			},
		),
		Entry("Add a single TLS sink with skip verify set",
			[]string{"add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345, EnableTLS: true, InsecureSkipVerify: true},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"test-ns\",\"tls\":{\"insecure_skip_verify\":true}}]\n    ClusterSinks []\n",
			},
		),
		Entry("Add multiple sinks",
			[]string{"add", "add"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "test.com", Port: 4567},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"test-ns\"}]\n    ClusterSinks []\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"test.com:4567\",\"namespace\":\"test-ns\"}]\n    ClusterSinks []\n",
			},
		),
		Entry("Delete sink",
			[]string{"add", "delete"},
			[]v1alpha1.SinkSpec{
				{Type: "syslog", Host: "example.com", Port: 12345},
				{Type: "syslog", Host: "example.com", Port: 12345},
			},
			[]string{
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"test-ns\"}]\n    ClusterSinks []\n",
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
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"test-ns\"}]\n    ClusterSinks []\n",
				"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12346\",\"namespace\":\"test-ns\"}]\n    ClusterSinks []\n",
			},
		),
	)

	It("doesn't update when there are no changes to the sink", func() {
		spyPatcher := &spyConfigMapPatcher{}
		spyDeleter := &spyDaemonSetPodDeleter{}
		c := sink.NewController(
			spyPatcher,
			spyDeleter,
			sink.NewConfig(),
		)

		s1 := &v1alpha1.LogSink{
			Spec: v1alpha1.SinkSpec{
				Type: "syslog",
				Host: "example.com",
				Port: 12345,
			},
		}
		s2 := &v1alpha1.LogSink{
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
		c := sink.NewController(
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

	It("it converts unspecified namespace to default", func() {
		spyPatcher := &spyConfigMapPatcher{}
		c := sink.NewController(
			spyPatcher,
			&spyDaemonSetPodDeleter{},
			sink.NewConfig(),
		)
		s1 := &v1alpha1.LogSink{
			Spec: v1alpha1.SinkSpec{
				Type: "syslog",
				Host: "example.com",
				Port: 12345,
			},
		}

		c.OnAdd(s1)

		spyPatcher.expectPatches([]string{
			"\n[OUTPUT]\n    Name syslog\n    Match *\n    Sinks [{\"addr\":\"example.com:12345\",\"namespace\":\"default\"}]\n    ClusterSinks []\n",
		})
	})
})

type jsonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type patch struct {
	name string
	pt   types.PatchType
	data []byte
}

type spyConfigMapPatcher struct {
	patchCalled bool
	patches     []patch
}

func (s *spyConfigMapPatcher) Patch(
	name string,
	pt types.PatchType,
	data []byte,
	subresources ...string,
) (*coreV1.ConfigMap, error) {
	s.patchCalled = true
	s.patches = append(s.patches, patch{
		name: name,
		pt:   pt,
		data: data,
	})
	return nil, nil
}

func (s *spyConfigMapPatcher) expectPatches(patches []string) {
	for i, p := range patches {
		jp := jsonPatch{
			Op:    "replace",
			Path:  "/data/outputs.conf",
			Value: p,
		}
		b, err := json.Marshal([]jsonPatch{jp})
		Expect(err).ToNot(HaveOccurred())

		Expect(s.patches[i].name).To(Equal(sink.ConfigMapName))
		Expect(s.patches[i].pt).To(Equal(types.JSONPatchType))
		Expect(s.patches[i].data).To(MatchJSON(b))
	}
}

type spyDaemonSetPodDeleter struct {
	deleteCollectionCalled bool
	Selector               string
}

func (s *spyDaemonSetPodDeleter) DeleteCollection(
	options *metav1.DeleteOptions,
	listOptions metav1.ListOptions,
) error {
	s.deleteCollectionCalled = true
	s.Selector = listOptions.LabelSelector
	return nil
}
