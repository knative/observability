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
	"github.com/knative/pkg/test/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterLogSink(t *testing.T) {
	const prefix = "cluster-log-sink-"

	logger := logging.GetContextLogger("TestClusterLogSink")
	clients, err := newClients()
	assertErr(t, "Error creating newClients: %v", err)

	createClusterLogSink(t, logger, prefix, clients.sinkClient)
	createSyslogReceiver(t, logger, prefix, clients.kubeClient)
	waitForFluentBitToBeReady(t, logger, prefix, clients.kubeClient)
	emitLogs(t, logger, prefix, clients.kubeClient)
	assertTheLogsGotThere(t, logger, prefix, clients.kubeClient)
}

func createClusterLogSink(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	sc sinkClient,
) {
	logger.Info("Creating the ClusterLogSink")
	_, err := sc.ClusterLogSink.Create(&v1alpha1.ClusterLogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "test",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: prefix + "syslog-receiver." + observabilityTestNamespace,
			Port: 24903,
		},
	})
	assertErr(t, "Error creating ClusterLogSink: %v", err)
}
