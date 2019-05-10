// +build e2e

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

package e2e

import (
	"testing"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	observabilityv1alpha1 "github.com/knative/observability/pkg/client/clientset/versioned/typed/sink/v1alpha1"
	"github.com/knative/pkg/test/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterMetricSink(t *testing.T) {
	var prefix = randomTestPrefix("cluster-metric-sink-")

	clients, logger := initialize(t, "TestClusterMetricSink")
	defer teardownNamespaces(clients, logger)

	logger.Infof("Test Prefix: %s", prefix)
	cleanup := createClusterMetricSink(t, logger, prefix, clients.sinkClient, observabilityTestNamespace)
	defer cleanup()

	waitForTelegrafToBeReady(t, logger, prefix, "telegraf", "knative-observability", clients.kubeClient)
	assertTelegrafOutputtedData(
		t,
		logger,
		"app=telegraf",
		"knative-observability",
		clients.kubeClient,
		clients.restCfg,
	)
}

func createClusterMetricSink(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	sc observabilityv1alpha1.ObservabilityV1alpha1Interface,
	namespace string,
) func() error {
	name := prefix + "test"
	logger.Info("Creating the ClusterMetricSink")
	_, err := sc.ClusterMetricSinks(namespace).Create(&v1alpha1.ClusterMetricSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.MetricSinkSpec{
			Inputs: []v1alpha1.MetricSinkMap{
				{
					"type":          "exec",
					"commands":      []string{"echo 5"},
					"data_format":   "value",
					"data_type":     "integer",
					"name_override": "test",
				},
			},
			Outputs: []v1alpha1.MetricSinkMap{
				{
					"type":        "file",
					"files":       []string{"/tmp/test"},
					"data_format": "json",
				},
			},
		},
	})
	assertErr(t, "Error creating ClusterMetricSink: %v", err)
	return func() error {
		return sc.ClusterMetricSinks(namespace).Delete(name, &metav1.DeleteOptions{})
	}
}
