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
	"fmt"
	"testing"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	observabilityv1alpha1 "github.com/knative/observability/pkg/client/clientset/versioned/typed/sink/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyslogClusterLogSink(t *testing.T) {
	prefix := randomTestPrefix("cluster-syslog-log-sink-")

	clients := initialize(t)
	defer teardownNamespaces(t, clients)

	t.Logf("Test Prefix: %s", prefix)
	cleanup := createClusterLogSink(t, prefix, clients.sinkClient, observabilityTestNamespace)
	defer cleanup()
	createSyslogReceiver(t, prefix, clients.kubeClient, observabilityTestNamespace)
	waitForFluentBitToBeReady(t, prefix, clients.kubeClient)
	emitLogs(t, prefix, clients.kubeClient, observabilityTestNamespace)
	emitLogs(t, prefix, clients.kubeClient, crosstalkTestNamespace)

	assertOnCrosstalk(t, prefix, clients, observabilityTestNamespace, func(m ReceiverMetrics) error {
		if m.Cluster != 20 {
			return fmt.Errorf("cluster count != 20")
		}
		messagesObservability, ok := m.Namespaced[observabilityTestNamespace]
		if !ok || messagesObservability != 10 {
			return fmt.Errorf("test namespace count != 10")
		}
		messagesCrosstalk, ok := m.Namespaced[crosstalkTestNamespace]
		if !ok || messagesCrosstalk != 10 {
			return fmt.Errorf("crosstalk namespace messages != 10")
		}
		return nil
	})
}

func TestClusterEventsLogSink(t *testing.T) {
	prefix := randomTestPrefix("cluster-event-log-sink-")

	clients := initialize(t)
	defer teardownNamespaces(t, clients)

	t.Logf("Test Prefix: %s", prefix)
	cleanup := createClusterLogSink(t, prefix, clients.sinkClient, observabilityTestNamespace)
	defer cleanup()
	createSyslogReceiver(t, prefix, clients.kubeClient, observabilityTestNamespace)
	waitForFluentBitToBeReady(t, prefix, clients.kubeClient)
	numEvents := 100
	emitEvents(t, "clearing-event-controller", clients.kubeClient, observabilityTestNamespace, numEvents)
	emitEvents(t, prefix, clients.kubeClient, observabilityTestNamespace, numEvents)
	emitEvents(t, prefix, clients.kubeClient, crosstalkTestNamespace, numEvents)
	assertOnCrosstalk(t, prefix, clients, observabilityTestNamespace, func(m ReceiverMetrics) error {
		if m.Cluster != 2*numEvents {
			return fmt.Errorf("cluster numEvents != %d", 2*numEvents)
		}
		messagesObservability, ok := m.Namespaced[observabilityTestNamespace]
		if !ok || messagesObservability != numEvents {
			return fmt.Errorf("test namespace numEvents != %d", numEvents)
		}
		messagesCrosstalk, ok := m.Namespaced[crosstalkTestNamespace]
		if !ok || messagesCrosstalk != numEvents {
			return fmt.Errorf("crosstalk namespace messages != %d", numEvents)
		}
		return nil
	})
}

func TestClusterWebhookLogSink(t *testing.T) {
	prefix := randomTestPrefix("cluster-webhook-log-sink-")

	clients := initialize(t)
	defer teardownNamespaces(t, clients)

	cleanup := createClusterWebhookLogSink(t, prefix, clients.sinkClient, observabilityTestNamespace)
	defer cleanup()
	createSyslogReceiver(t, prefix, clients.kubeClient, observabilityTestNamespace)
	waitForFluentBitToBeReady(t, prefix, clients.kubeClient)
	emitLogs(t, prefix, clients.kubeClient, observabilityTestNamespace)
	emitLogs(t, prefix, clients.kubeClient, crosstalkTestNamespace)
	assertOnCrosstalk(t, prefix, clients, observabilityTestNamespace, func(m ReceiverMetrics) error {
		messagesObservability, ok := m.WebhookNamespaced[observabilityTestNamespace]
		if !ok || messagesObservability < 10 {
			return fmt.Errorf("test namespace count < 10")
		}
		messagesCrosstalk, ok := m.WebhookNamespaced[crosstalkTestNamespace]
		if !ok || messagesCrosstalk < 10 {
			return fmt.Errorf("crosstalk namespace < 10")
		}
		return nil
	},
	)
}

func createClusterLogSink(
	t *testing.T,
	prefix string,
	sc observabilityv1alpha1.ObservabilityV1alpha1Interface,
	namespace string,
) func() error {
	name := prefix + "test"
	t.Log("Creating the ClusterLogSink")
	_, err := sc.ClusterLogSinks(namespace).Create(&v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			SyslogSpec: v1alpha1.SyslogSpec{
				Host:      prefix + "syslog-receiver." + observabilityTestNamespace,
				Port:      24903,
				EnableTLS: true,
			},
			InsecureSkipVerify: true,
		},
	})
	assertErr(t, "Error creating ClusterLogSink: %v", err)

	return func() error {
		return sc.ClusterLogSinks(namespace).Delete(name, &metav1.DeleteOptions{})
	}
}

func createClusterWebhookLogSink(
	t *testing.T,
	prefix string,
	sc observabilityv1alpha1.ObservabilityV1alpha1Interface,
	namespace string,
) func() error {
	name := prefix + "test"
	t.Log("Creating the ClusterLogSink")
	_, err := sc.ClusterLogSinks(namespace).Create(&v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.SinkSpec{
			Type: "webhook",
			WebhookSpec: v1alpha1.WebhookSpec{
				URL: "https://" + prefix + "syslog-receiver." + observabilityTestNamespace + ":7070/webhook",
			},
			InsecureSkipVerify: true,
		},
	})
	assertErr(t, "Error creating ClusterLogSink: %v", err)

	return func() error {
		return sc.ClusterLogSinks(namespace).Delete(name, &metav1.DeleteOptions{})
	}
}
