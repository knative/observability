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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	observabilityv1alpha1 "github.com/knative/observability/pkg/client/clientset/versioned/typed/sink/v1alpha1"
	"github.com/knative/pkg/test"
	"github.com/knative/pkg/test/monitoring"
)

func TestMetricSink(t *testing.T) {
	t.Run("uses the same image as the telegraf daemonset", func(t *testing.T) {
		// The metric controller deploys a namespaced telegraf deployment
		// whereas the telegraf daemonset is deployed via the manifest. This
		// test ensures the metric controller deploys the correct version of
		// telegraf.
		var prefix = randomTestPrefix("metric-sink-")

		clients := initialize(t)
		defer teardownNamespaces(t, clients)

		t.Logf("Test Prefix: %s", prefix)
		msName := prefix + "test"
		resName := "telegraf-" + msName

		cleanup := createMetricSink(
			t,
			msName,
			observabilityTestNamespace,
			clients.sinkClient,
		)
		defer cleanup()

		waitForTelegrafToBeReady(
			t,
			prefix,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)

		dspods, err := monitoring.GetPods(clients.kubeClient.Kube, "telegraf", "knative-observability")
		assertErr(t, "unable to get telegraf daemonset pods", err)
		daemonSetImage := dspods.Items[0].Spec.Containers[0].Image

		dpods, err := monitoring.GetPods(clients.kubeClient.Kube, resName, observabilityTestNamespace)
		assertErr(t, "unable to get telegraf deployment pods", err)
		deploymentImage := dpods.Items[0].Spec.Containers[0].Image

		if daemonSetImage != deploymentImage {
			t.Fatal("Telegraf daemon set and deployments are not using the same image")
		}

	})

	t.Run("creates a metric sink in the specified namespace", func(t *testing.T) {
		var prefix = randomTestPrefix("metric-sink-")

		clients := initialize(t)
		defer teardownNamespaces(t, clients)

		t.Logf("Test Prefix: %s", prefix)
		msName := prefix + "test"
		resName := "telegraf-" + msName
		prometheusMetricName := strings.ReplaceAll(prefix, "-", "_") + "test_metric"

		cleanup := createMetricSink(
			t,
			msName,
			observabilityTestNamespace,
			clients.sinkClient,
		)
		defer cleanup()

		assertRoleExists(
			t,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		assertRoleBindingExists(
			t,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		assertConfigMapExists(
			t,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		createPrometheusScrapeTarget(
			t,
			prometheusMetricName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		waitForTelegrafToBeReady(
			t,
			prefix,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		assertTelegrafOutputtedData(
			t,
			"app="+resName,
			observabilityTestNamespace,
			clients.kubeClient,
			clients.restCfg,
			func(metrics map[string]float64) []error {
				return checkMetrics(metrics, map[string]float64{
					"test":               5,
					prometheusMetricName: 105, // This value is hardcoded in the prometheus_scrape_target docker image
				})
			},
		)
	})
}

func createMetricSink(
	t *testing.T,
	name string,
	namespace string,
	sc observabilityv1alpha1.ObservabilityV1alpha1Interface,
) func() error {
	t.Log("Creating the MetricSink")
	_, err := sc.MetricSinks(namespace).Create(&v1alpha1.MetricSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
	assertErr(t, "Error creating MetricSink: %v", err)
	return func() error {
		return sc.MetricSinks(namespace).Delete(name, &metav1.DeleteOptions{})
	}
}

func assertConfigMapExists(
	t *testing.T,
	name string,
	namespace string,
	kc *test.KubeClient) {

	var (
		err error
		cm  *corev1.ConfigMap
	)

	t.Logf("Verifying existence of config map in %s", namespace)
	for i := 0; i < 300; i++ {
		cm, err = kc.Kube.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
		if cm.Name == name {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("Could not verify config map: %s", err)
}

func assertRoleExists(
	t *testing.T,
	name string,
	namespace string,
	kc *test.KubeClient) {

	var (
		err error
		r   *rbacv1.Role
	)

	t.Logf("Verifying existence of role in %s", namespace)
	for i := 0; i < 300; i++ {
		r, err = kc.Kube.RbacV1().Roles(namespace).Get(name, metav1.GetOptions{})
		if r.Name == name {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}

	t.Fatalf("Could not verify role: %s", err)
}

func assertRoleBindingExists(
	t *testing.T,
	name string,
	namespace string,
	kc *test.KubeClient) {

	var (
		err error
		rb  *rbacv1.RoleBinding
	)

	t.Logf("Verifying existence of role binding in %s", namespace)
	for i := 0; i < 300; i++ {
		rb, err = kc.Kube.RbacV1().RoleBindings(namespace).Get(name, metav1.GetOptions{})
		if rb.Name == name {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("Could not verify role binding: %s", err)
}
