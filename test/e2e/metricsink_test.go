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
	"time"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	observabilityv1alpha1 "github.com/knative/observability/pkg/client/clientset/versioned/typed/sink/v1alpha1"
	"github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
)

func TestMetricSink(t *testing.T) {
	t.Run("creates a metric sink in the specified namespace", func(t *testing.T) {
		var prefix = randomTestPrefix("metric-sink-")

		clients, logger := initialize(t)
		defer teardownNamespaces(clients, logger)

		logger.Infof("Test Prefix: %s", prefix)
		msName := prefix + "test"
		resName := "telegraf-" + msName

		cleanup := createMetricSink(
			t,
			logger,
			msName,
			observabilityTestNamespace,
			clients.sinkClient,
		)
		defer cleanup()

		assertRoleExists(
			t,
			logger,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		assertRoleBindingExists(
			t,
			logger,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		assertConfigMapExists(
			t,
			logger,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		waitForTelegrafToBeReady(
			t,
			logger,
			prefix,
			resName,
			observabilityTestNamespace,
			clients.kubeClient,
		)
		assertTelegrafOutputtedData(
			t,
			logger,
			"app="+resName,
			observabilityTestNamespace,
			clients.kubeClient,
			clients.restCfg,
		)
	})
}

func createMetricSink(
	t *testing.T,
	logger *logging.BaseLogger,
	name string,
	namespace string,
	sc observabilityv1alpha1.ObservabilityV1alpha1Interface,
) func() error {
	logger.Info("Creating the MetricSink")
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
	logger *logging.BaseLogger,
	name string,
	namespace string,
	kc *test.KubeClient) {

	var (
		err error
		cm  *v1.ConfigMap
	)

	logger.Infof("Verifying existence of config map in %s", namespace)
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
	logger *logging.BaseLogger,
	name string,
	namespace string,
	kc *test.KubeClient) {

	var (
		err error
		r   *rbacv1.Role
	)

	logger.Infof("Verifying existence of role in %s", namespace)
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
	logger *logging.BaseLogger,
	name string,
	namespace string,
	kc *test.KubeClient) {

	var (
		err error
		rb  *rbacv1.RoleBinding
	)

	logger.Infof("Verifying existence of role binding in %s", namespace)
	for i := 0; i < 300; i++ {
		rb, err = kc.Kube.RbacV1().RoleBindings(namespace).Get(name, metav1.GetOptions{})
		if rb.Name == name {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("Could not verify role binding: %s", err)
}
