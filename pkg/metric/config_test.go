package metric_test

import (
	"sync"
	"testing"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEmptyConfig(t *testing.T) {
	config := metric.NewConfig(false, "").String()

	const expected = `[inputs]

  [[inputs.cpu]]

[outputs]

  [[outputs.discard]]
`
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestSingleSink(t *testing.T) {
	sc := metric.NewConfig(false, "")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "cpu",
					"foo":  "bar",
					"baz":  1234,
				},
				{
					"type": "prometheus",
					"url":  "http://foobar",
				},
			},
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "datadog",
					"api_key": "some-key",
				},
			},
		},
	}

	sc.UpsertSink(sink)

	const expected = `[inputs]

  [[inputs.cpu]]
    baz = 1234
    foo = "bar"

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

  [[inputs.prometheus]]
    url = "http://foobar"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`

	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestClusterNameTag(t *testing.T) {
	sc := metric.NewConfig(false, "cluster-name")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "cpu",
					"foo":  "bar",
					"baz":  1234,
				},
				{
					"type": "prometheus",
					"url":  "http://foobar",
				},
			},
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "datadog",
					"api_key": "some-key",
				},
			},
		},
	}

	sc.UpsertSink(sink)

	const expected = `[global_tags]
  cluster_name = "cluster-name"

[inputs]

  [[inputs.cpu]]
    baz = 1234
    foo = "bar"

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

  [[inputs.prometheus]]
    url = "http://foobar"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`

	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestMultipleSinks(t *testing.T) {
	sc := metric.NewConfig(false, "")
	sink1 := v1alpha1.ClusterMetricSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-metric-sink-a",
		},
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "cpu",
					"baz":  1234,
				},
			},
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "influx",
					"api_key": "some-key-1",
				},
			},
		},
	}
	sink2 := v1alpha1.ClusterMetricSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-metric-sink-b",
		},
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "prometheus",
					"url":  "http://foobar",
				},
			},
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "datadog",
					"api_key": "some-key-2",
				},
			},
		},
	}

	sc.UpsertSink(sink1)
	_ = sc.String()
	sc.UpsertSink(sink2)

	const expected = `[inputs]

  [[inputs.cpu]]
    baz = 1234

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

  [[inputs.prometheus]]
    url = "http://foobar"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key-2"

  [[outputs.influx]]
    api_key = "some-key-1"
`
	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}
func TestDeleteSink(t *testing.T) {
	sc := metric.NewConfig(false, "")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "datadog",
					"api_key": "some-key",
				},
			},
		},
	}

	sc.UpsertSink(sink)

	sc.DeleteSink(sink)

	const expected = `[inputs]

  [[inputs.cpu]]

[outputs]

  [[outputs.discard]]
`
	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestMissingType(t *testing.T) {
	sc := metric.NewConfig(false, "")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"foo": "bar",
				},
				{
					"type": "cpu",
					"foo":  "bar",
				},
			},
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"api_key": "some-key",
				},
				{
					"api_key": "some-key",
					"type":    "datadog",
				},
			},
		},
	}

	sc.UpsertSink(sink)

	const expected = `[inputs]

  [[inputs.cpu]]
    foo = "bar"

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`

	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestInputOnlyConfig(t *testing.T) {
	sc := metric.NewConfig(false, "")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "cpu",
					"foo":  "bar",
					"baz":  1234,
				},
			},
		},
	}

	sc.UpsertSink(sink)

	const expected = `[inputs]

  [[inputs.cpu]]

[outputs]

  [[outputs.discard]]
`
	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestOutputOnlyConfig(t *testing.T) {
	sc := metric.NewConfig(false, "")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "datadog",
					"api_key": "some-key",
				},
			},
		},
	}

	sc.UpsertSink(sink)

	const expected = `[inputs]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`
	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}

func TestUseInsecureKubernetesPortConfig(t *testing.T) {
	sc := metric.NewConfig(true, "")
	sink := v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":    "datadog",
					"api_key": "some-key",
				},
			},
		},
	}

	sc.UpsertSink(sink)

	const expected = `[inputs]

  [[inputs.kubernetes]]
    url = "http://127.0.0.1:10255"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`
	config := sc.String()
	if config != expected {
		t.Errorf(
			"Config does not match:\nExpected: %s\nActual: %s",
			expected,
			config,
		)
	}
}
func TestConcurrentAccess(t *testing.T) {
	sc := metric.NewConfig(false, "")
	wg := &sync.WaitGroup{}

	const count = 100
	wg.Add(3 * count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			sc.UpsertSink(v1alpha1.ClusterMetricSink{})
		}()
		go func() {
			defer wg.Done()
			_ = sc.String()
		}()
		go func() {
			defer wg.Done()
			sc.DeleteSink(v1alpha1.ClusterMetricSink{})
		}()
	}

	wg.Wait()
}
