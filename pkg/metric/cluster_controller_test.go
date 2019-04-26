package metric_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/metric"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestClusterSinkModification(t *testing.T) {
	var tests = []struct {
		name       string
		operations []string
		specs      []v1alpha1.MetricSinkSpec
		patches    []string
	}{
		{
			"Add a single sink",
			[]string{"add"},
			[]v1alpha1.MetricSinkSpec{
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type":    "datadog",
							"api_key": "some-key",
						},
					},
				},
			},
			[]string{
				`[inputs]

  [[inputs.cpu]]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`,
			},
		},
		{
			"No sink inputs",
			[]string{"add"},
			[]v1alpha1.MetricSinkSpec{
				{
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type":    "datadog",
							"api_key": "some-key",
						},
					},
				},
			},
			[]string{
				`[inputs]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`,
			},
		},
		{
			"Add sink multiple times",
			[]string{"add", "add"},
			[]v1alpha1.MetricSinkSpec{
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type":    "datadog",
							"api_key": "some-key",
						},
					},
				},
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type": "prometheus",
							"url":  "example.com",
						},
					},
				},
			},
			[]string{
				`[inputs]

  [[inputs.cpu]]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`,
				`[inputs]

  [[inputs.cpu]]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.prometheus]]
    url = "example.com"
`,
			},
		},
		{
			"Update sink",
			[]string{"add", "update"},
			[]v1alpha1.MetricSinkSpec{
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type":    "datadog",
							"api_key": "some-key",
						},
					},
				},
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type": "prometheus",
							"url":  "example.com",
						},
					},
				},
			},
			[]string{
				`[inputs]

  [[inputs.cpu]]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`,
				`[inputs]

  [[inputs.cpu]]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.prometheus]]
    url = "example.com"
`,
			},
		},
		{
			"Delete sink",
			[]string{"add", "delete"},
			[]v1alpha1.MetricSinkSpec{
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type":    "datadog",
							"api_key": "some-key",
						},
					},
				},
				{
					Inputs: []v1alpha1.MetricSinkMap{
						{
							"type": "cpu",
						},
					},
					Outputs: []v1alpha1.MetricSinkMap{
						{
							"type":    "datadog",
							"api_key": "some-key",
						},
					},
				},
			},
			[]string{
				`[inputs]

  [[inputs.cpu]]

  [[inputs.kubernetes]]
    bearer_token = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    insecure_skip_verify = true
    url = "https://127.0.0.1:10250"

[outputs]

  [[outputs.datadog]]
    api_key = "some-key"
`, `[inputs]

  [[inputs.cpu]]

[outputs]

  [[outputs.discard]]
`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapPatcher := &spyConfigMapPatcher{}
			podDeleter := &spyDeploymentPodDeleter{}

			c := metric.NewClusterController(mapPatcher, podDeleter, metric.NewConfig(false, ""))
			for i, spec := range test.specs {
				d := &v1alpha1.ClusterMetricSink{
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
			mapPatcher.expectPatches(test.patches, t)
			if podDeleter.Selector != "app=telegraf" {
				t.Errorf("DaemonSet PodDeleter not equal: Expected: %s, Actual: %s", podDeleter.Selector, "app=telegraf")
			}
		})
	}
}

func TestNoopChange(t *testing.T) {
	mapPatcher := &spyConfigMapPatcher{}
	podDeleter := &spyDeploymentPodDeleter{}
	c := metric.NewClusterController(mapPatcher, podDeleter, metric.NewConfig(false, ""))

	s1 := &v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "cpu",
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
	s2 := &v1alpha1.ClusterMetricSink{
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type": "cpu",
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

	c.OnUpdate(s1, s2)
	if mapPatcher.patchCalled {
		t.Errorf("Expected patch to not be called")
	}
	if podDeleter.deleteCollectionCalled {
		t.Errorf("Expected delete to not be called")
	}
}

func TestBadInputs(t *testing.T) {
	c := metric.NewClusterController(
		&spyConfigMapPatcher{},
		&spyDeploymentPodDeleter{},
		metric.NewConfig(false, ""),
	)
	//shouldn't panic
	c.OnAdd("")
	c.OnDelete(1)
	c.OnUpdate(nil, nil)
}

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

func (s *spyConfigMapPatcher) expectPatches(patches []string, t *testing.T) {
	for i, p := range patches {
		if len(s.patches) <= i {
			t.Errorf("Missing patch %d", i)
			continue
		}

		if s.patches[i].name != "telegraf" {
			t.Errorf("Sink map name does not equal Got: %s, Expected %s", s.patches[i].name, "telegraf")
		}

		if s.patches[i].pt != types.JSONPatchType {
			t.Errorf("Patch Type does not equal Got: %s, Expected %s", s.patches[i].pt, types.JSONPatchType)
		}

		jpExpected := []jsonPatch{
			{
				Op:    "replace",
				Path:  "/data/cluster-metric-sinks.conf",
				Value: p,
			},
		}
		var jpActual []jsonPatch
		err := json.Unmarshal(s.patches[i].data, &jpActual)
		if err != nil {
			t.Errorf("Could not Unmarshal json patch: %s", err)
		}

		if diff := cmp.Diff(jpExpected, jpActual); diff != "" {
			t.Errorf("Patches not equal (-want, +got) = %v", diff)
		}
	}
}

type spyDeploymentPodDeleter struct {
	deleteCollectionCalled bool
	Selector               string
}

func (s *spyDeploymentPodDeleter) DeleteCollection(
	options *metav1.DeleteOptions,
	listOptions metav1.ListOptions,
) error {
	s.deleteCollectionCalled = true
	s.Selector = listOptions.LabelSelector
	return nil
}
