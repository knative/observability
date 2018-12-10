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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Config", func() {
	It("renders a null output when empty", func() {
		Expect(sink.NewConfig().String()).To(Equal(`
[OUTPUT]
    Name null
    Match *
`))
	})

	It("renders a single sink", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "some-namespace"
					}
				]`),
				"ClusterSinks": MatchJSON(`[]`),
			},
		)
	})

	It("renders multiple sinks", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "ns1"
					},
					{
						"addr": "example.org:45678",
						"namespace": "ns2"
					}
				]`),
			},
		)
	})

	It("renders multiple cluster sinks", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[]`),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "example.com:12345"
					},
					{
						"addr": "example.org:45678"
					}
				]`),
			},
		)
	})

	It("renders sinks and cluster sinks together", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "ns1"
					}
				]`),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "example.org:45678"
					}
				]`),
			},
		)
	})

	It("returns null config when all sinks are removed", func() {
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

		Expect(sc.String()).To(Equal(`
[OUTPUT]
    Name null
    Match *
`))
	})

	It("can remove a sink", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "example2.com:12345"
					}
				]`),
				"Sinks": MatchJSON(`[]`),
			},
		)
	})

	It("can remove cluster sinks", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "ns1"
					}
				]`),
				"ClusterSinks": MatchJSON(`[]`),
			},
		)
	})

	It("can update sinks", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "ns.sample.com:12345",
						"namespace": "ns1"
					}
				]`),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "cl.sample.org:45678"
					}
				]`),
			},
		)
	})

	It("can concurrently update sinks", func() {
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
		Eventually(done).Should(BeClosed())

		expectConfig(
			sc.String(),
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "ns1"
					}
				]`),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "example.org:45678"
					}
				]`),
			},
		)

		done = make(chan struct{})
		go func() {
			defer close(done)
			sc.DeleteClusterSink(s2)
		}()
		sc.DeleteSink(s1)
		Eventually(done).Should(BeClosed())
		Expect(sc.String()).To(Equal(`
[OUTPUT]
    Name null
    Match *
`))
	})

	It("orders the sinks by namespace and name", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "a-ns1"
					},
					{
						"addr": "example.com:12345",
						"namespace": "default"
					},
					{
						"addr": "example.org:45678",
						"namespace": "z-ns2"
					},
					{
						"addr": "example.org:12345",
						"namespace": "z-ns2"
					}
				]`),
			},
		)
	})

	It("encodes tls options", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "some-namespace",
						"tls": {}
					}
				]`),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"tls": {}
					}
				]`),
			},
		)

		s1.Spec.InsecureSkipVerify = true
		s2.Spec.InsecureSkipVerify = true

		sc.UpsertSink(s1)
		sc.UpsertClusterSink(s2)

		expectConfig(
			sc.String(),
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "some-namespace",
						"tls": {
							"insecure_skip_verify": true
						}
					}
				]`),
				"ClusterSinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"tls": {
							"insecure_skip_verify": true
						}
					}
				]`),
			},
		)
	})

	It("converts empty namespace to default namespace", func() {
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
			map[string]types.GomegaMatcher{
				"Name":  Equal("syslog"),
				"Match": Equal("*"),
				"Sinks": MatchJSON(`[
					{
						"addr": "example.com:12345",
						"namespace": "default"
					}
				]`),
			},
		)
	})
})

func expectConfig(conf string, matchers map[string]types.GomegaMatcher) {
	conf = strings.TrimSpace(conf)

	ExpectWithOffset(1, conf).To(HavePrefix("[OUTPUT]\n"))

	conf = strings.TrimPrefix(conf, "[OUTPUT]\n")
	lines := strings.Split(conf, "\n")
	props := make(map[string]string, len(lines))
	for _, line := range lines {
		kv := strings.Split(strings.TrimSpace(line), " ")
		props[kv[0]] = kv[1]
	}
	for k, m := range matchers {
		actualValue, ok := props[k]
		if !ok {
			Fail(fmt.Sprintf("matcher provided for missing value %s", k), 1)
		}
		ExpectWithOffset(1, actualValue).To(m)
	}
}
