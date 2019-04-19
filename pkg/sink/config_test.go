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
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/sink"
	"github.com/knative/observability/pkg/sink/flbconfig"
)

var emptyConfig = `
[OUTPUT]
    Name null
    Match *
    StatsAddr 127.0.0.1:5000
`

func TestEmptyConfig(t *testing.T) {
	config := sink.NewConfig("127.0.0.1:5000").String()
	if config != emptyConfig {
		t.Errorf("Empty Config not equal: Expected: %s Actual: %s", emptyConfig, config)
	}
}

func TestSingleSink(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	sink := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name",
			Namespace: "some-namespace",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}
	sc.UpsertSink(sink)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name",
				Addr:      "example.com:12345",
				Namespace: "some-namespace",
			},
		},
		[]clusterSink{},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestNoHostname(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	sink := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name",
			Namespace: "some-namespace",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}
	sc.UpsertSink(sink)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name",
				Addr:      "example.com:12345",
				Namespace: "some-namespace",
			},
		},
		[]clusterSink{},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestMultipleSinks(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}
	s2 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-2",
			Namespace: "ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 45678,
			},
		},
	}
	sc.UpsertSink(s1)
	sc.UpsertSink(s2)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "example.com:12345",
				Namespace: "ns1",
			},
			{
				Name:      "some-name-2",
				Addr:      "example.org:45678",
				Namespace: "ns2",
			},
		},
		[]clusterSink{},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestMultipleClusterSinks(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-1",
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
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 45678,
			},
		},
	}

	sc.UpsertClusterSink(s1)
	sc.UpsertClusterSink(s2)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{},
		[]clusterSink{
			{
				Name: "some-name-1",
				Addr: "example.com:12345",
			},
			{
				Name: "some-name-2",
				Addr: "example.org:45678",
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestSinksWithClusterSinks(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
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
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 45678,
			},
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "example.com:12345",
				Namespace: "ns1",
			},
		},
		[]clusterSink{
			{
				Name: "some-name-2",
				Addr: "example.org:45678",
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestAllSinksRemoved(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "ns.example.com",
				Port: 12345,
			},
		},
	}
	cs := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "cl.example.org",
				Port: 45678,
			},
		},
	}

	sc.UpsertSink(s)
	sc.UpsertClusterSink(cs)
	sc.DeleteSink(s)
	sc.DeleteClusterSink(cs)

	if sc.String() != emptyConfig {
		t.Errorf(
			"Empty Config not equal: Expected: %s Actual: %s",
			emptyConfig,
			sc.String(),
		)
	}
}

func TestRemoveSink(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "some-namespace-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example1.com",
				Port: 12345,
			},
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example2.com",
				Port: 12345,
			},
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)
	sc.DeleteSink(s1)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{},
		[]clusterSink{
			{
				Name: "some-name-2",
				Addr: "example2.com:12345",
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestRemoveClusterSink(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
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
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 45678,
			},
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)
	sc.DeleteClusterSink(s2)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "example.com:12345",
				Namespace: "ns1",
			},
		},
		[]clusterSink{},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestUpdateSink(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "ns.example.com",
				Port: 12345,
			},
		},
	}
	cs := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "cl.example.org",
				Port: 45678,
			},
		},
	}

	sc.UpsertSink(s)
	sc.UpsertClusterSink(cs)
	s.Spec.Host = "ns.sample.com"
	cs.Spec.Host = "cl.sample.org"
	sc.UpsertSink(s)
	sc.UpsertClusterSink(cs)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "ns.sample.com:12345",
				Namespace: "ns1",
			},
		},
		[]clusterSink{
			{
				Name: "some-name-1",
				Addr: "cl.sample.org:45678",
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestUpdateConcurrency(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "ns1",
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
			Name:      "some-name-2",
			Namespace: "ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 45678,
			},
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

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "example.com:12345",
				Namespace: "ns1",
			},
		},
		[]clusterSink{
			{
				Name: "some-name-2",
				Addr: "example.org:45678",
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}

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
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-3",
			Namespace: "a-ns1",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}
	s2 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-4",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}
	s3 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "z-ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 45678,
			},
		},
	}
	s4 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-2",
			Namespace: "z-ns2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.org",
				Port: 12345,
			},
		},
	}

	sc.UpsertSink(s4)
	sc.UpsertSink(s3)
	sc.UpsertSink(s2)
	sc.UpsertSink(s1)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-3",
				Addr:      "example.com:12345",
				Namespace: "a-ns1",
			},
			{
				Name:      "some-name-4",
				Addr:      "example.com:12345",
				Namespace: "default",
			},
			{
				Name:      "some-name-1",
				Addr:      "example.org:45678",
				Namespace: "z-ns2",
			},
			{
				Name:      "some-name-2",
				Addr:      "example.org:12345",
				Namespace: "z-ns2",
			},
		},
		[]clusterSink{},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestTlsEncoding(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	s1 := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-1",
			Namespace: "some-namespace",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host:      "example.com",
				Port:      12345,
				EnableTLS: true,
			},
		},
	}
	s2 := &v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name-2",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host:      "example.com",
				Port:      12345,
				EnableTLS: true,
			},
		},
	}

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "example.com:12345",
				Namespace: "some-namespace",
				TLS:       &tlsConfig{},
			},
		},
		[]clusterSink{
			{
				Name: "some-name-2",
				Addr: "example.com:12345",
				TLS:  &tlsConfig{},
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}

	s1.Spec.InsecureSkipVerify = true
	s2.Spec.InsecureSkipVerify = true

	sc.UpsertSink(s1)
	sc.UpsertClusterSink(s2)

	config = sc.String()

	f, err = flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig = sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name-1",
				Addr:      "example.com:12345",
				Namespace: "some-namespace",
				TLS: &tlsConfig{
					InsecureSkipVerify: true,
				},
			},
		},
		[]clusterSink{
			{
				Name: "some-name-2",
				Addr: "example.com:12345",
				TLS: &tlsConfig{
					InsecureSkipVerify: true,
				},
			},
		},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestEmptyNamespace(t *testing.T) {
	sc := sink.NewConfig("127.0.0.1:5000")
	sink := &v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host: "example.com",
				Port: 12345,
			},
		},
	}

	sc.UpsertSink(sink)

	config := sc.String()

	f, err := flbconfig.Parse("", config)
	if err != nil {
		t.Fatal(err)
	}
	expectedConfig := sinksToConfigAST(
		t,
		[]namespaceSink{
			{
				Name:      "some-name",
				Addr:      "example.com:12345",
				Namespace: "default",
			},
		},
		[]clusterSink{},
	)
	if !cmp.Equal(f, expectedConfig) {
		t.Fatal(cmp.Diff(f, expectedConfig))
	}
}

func TestWebhookSinks(t *testing.T) {
	testCases := map[string]struct {
		logSinks        []*v1alpha1.LogSink
		clusterLogSinks []*v1alpha1.ClusterLogSink
		expectedConfig  flbconfig.File
	}{
		"namespaced with https": {
			logSinks: []*v1alpha1.LogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "https://example.com/some/path",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*_some-namespace_*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "443",
						},
						{
							Key:   "URI",
							Value: "/some/path",
						},
						{
							Key:   "tls",
							Value: "On",
						},
					},
				},
			),
		},
		"namespace with http URL": {
			logSinks: []*v1alpha1.LogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "http://example.com/some/path",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*_some-namespace_*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "80",
						},
						{
							Key:   "URI",
							Value: "/some/path",
						},
					},
				},
			),
		},
		"namespace with custom port": {
			logSinks: []*v1alpha1.LogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "http://example.com:12345/some/path",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*_some-namespace_*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "12345",
						},
						{
							Key:   "URI",
							Value: "/some/path",
						},
					},
				},
			),
		},
		"namespace with multiple": {
			logSinks: []*v1alpha1.LogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name-1",
						Namespace: "some-namespace-1",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "http://example.com/some/path-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name-2",
						Namespace: "some-namespace-2",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "http://example.com/some/path-2",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*_some-namespace-1_*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "80",
						},
						{
							Key:   "URI",
							Value: "/some/path-1",
						},
					},
				},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*_some-namespace-2_*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "80",
						},
						{
							Key:   "URI",
							Value: "/some/path-2",
						},
					},
				},
			),
		},
		"cluster sink": {
			clusterLogSinks: []*v1alpha1.ClusterLogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "http://example.com/some/path",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "80",
						},
						{
							Key:   "URI",
							Value: "/some/path",
						},
					},
				},
			),
		},
		"ignore invalid URL": {
			clusterLogSinks: []*v1alpha1.ClusterLogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: ":@:@:@$",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
			),
		},
		"with URL that does not have a path": {
			logSinks: []*v1alpha1.LogSink{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
					Spec: v1alpha1.SinkSpec{
						Type: "webhook",
						WebhookSpec: v1alpha1.WebhookSpec{
							URL: "https://example.com",
						},
					},
				},
			},
			expectedConfig: sinksToConfigAST(
				t,
				[]namespaceSink{},
				[]clusterSink{},
				flbconfig.Section{
					Name: "OUTPUT",
					KeyValues: []flbconfig.KeyValue{
						{
							Key:   "Name",
							Value: "http",
						},
						{
							Key:   "Match",
							Value: "*_some-namespace_*",
						},
						{
							Key:   "Format",
							Value: "json",
						},
						{
							Key:   "Host",
							Value: "example.com",
						},
						{
							Key:   "Port",
							Value: "443",
						},
						{
							Key:   "URI",
							Value: "/",
						},
						{
							Key:   "tls",
							Value: "On",
						},
					},
				},
			),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sc := sink.NewConfig("127.0.0.1:5000")

			for _, s := range tc.logSinks {
				sc.UpsertSink(s)
			}
			for _, s := range tc.clusterLogSinks {
				sc.UpsertClusterSink(s)
			}

			config := sc.String()

			f, err := flbconfig.Parse("", config)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(f, tc.expectedConfig, compareFLBConfig) {
				t.Fatal(cmp.Diff(f, tc.expectedConfig))
			}
		})
	}
}

type clusterSink struct {
	Addr string     `json:"addr,omitempty"`
	TLS  *tlsConfig `json:"tls,omitempty"`
	Name string     `json:"name,omitempty"`
}

type namespaceSink struct {
	Addr      string     `json:"addr,omitempty"`
	Namespace string     `json:"namespace,omitempty"`
	TLS       *tlsConfig `json:"tls,omitempty"`
	Name      string     `json:"name,omitempty"`
}

type tlsConfig struct {
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`
}

var compareFLBConfig = cmp.Comparer(func(x, y flbconfig.File) bool {
	if x.Name != y.Name {
		fmt.Println("file names do not match")
		return false
	}
	if len(x.Sections) != len(y.Sections) {
		fmt.Printf(
			"section lengths differ: %d != %d\n",
			len(x.Sections),
			len(y.Sections),
		)
		return false
	}

	ySections := make([]flbconfig.Section, len(y.Sections))
	copy(ySections, y.Sections)

outer:
	for _, xs := range x.Sections {
		for i, ys := range ySections {
			if cmp.Equal(xs, ys) {
				ySections = append(ySections[:i], ySections[i+1:]...)
				continue outer
			}
		}
		fmt.Printf("section was missing: %#v\n", xs)
		return false
	}

	return true
})

func sinksToConfigAST(
	t *testing.T,
	nsSinks []namespaceSink,
	clSinks []clusterSink,
	sections ...flbconfig.Section,
) flbconfig.File {
	nsSinksJSON, err := json.Marshal(nsSinks)
	if err != nil {
		t.Fatal(err)
	}
	clSinksJSON, err := json.Marshal(clSinks)
	if err != nil {
		t.Fatal(err)
	}

	sections = append([]flbconfig.Section{{}}, sections...)

	if len(nsSinks) > 0 || len(clSinks) > 0 {
		section := flbconfig.Section{
			Name: "OUTPUT",
			KeyValues: []flbconfig.KeyValue{
				{
					Key:   "Name",
					Value: "syslog",
				},
				{
					Key:   "Match",
					Value: "*",
				},
				{
					Key:   "StatsAddr",
					Value: "127.0.0.1:5000",
				},
				{
					Key:   "Sinks",
					Value: string(nsSinksJSON),
				},
				{
					Key:   "ClusterSinks",
					Value: string(clSinksJSON),
				},
			},
		}

		sections = append(sections, section)
	}

	return flbconfig.File{
		Sections: sections,
	}
}
