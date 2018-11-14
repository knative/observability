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

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLogSink(t *testing.T) {
	const prefix = "log-sink-"

	clients, err := newClients()
	logger := logging.GetContextLogger("TestLogSink")
	if err != nil {
		t.Fatalf("Error creating newClients: %v", err)
	}

	logger.Info("Creating the log sink")
	_, err = clients.sinkClient.LogSink.Create(&v1alpha1.LogSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "test",
		},
		Spec: v1alpha1.SinkSpec{
			Type: "syslog",
			Host: prefix + "syslog-receiver." + observabilityTestNamespace,
			Port: 24903,
		},
	})
	if err != nil {
		t.Fatalf("Error creating LogSink: %v", err)
	}

	logger.Info("Creating the service for the syslog receiver")
	_, err = clients.kubeClient.Kube.Core().Services(observabilityTestNamespace).Create(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "syslog-receiver",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "syslog",
				Port: 24903,
			}, {
				Name: "metrics",
				Port: 6060,
			}},
			Selector: map[string]string{
				"app": prefix + "syslog-receiver",
			},
		},
	})
	if err != nil {
		t.Fatalf("Error creating LogSink: %v", err)
	}

	logger.Info("Creating the pod for the syslog receiver")
	_, err = clients.kubeClient.Kube.Core().Pods(observabilityTestNamespace).Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "syslog-receiver",
			Labels: map[string]string{
				"app": prefix + "syslog-receiver",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "syslog-receiver",
				Image: "oratos/crosstalk-receiver",
				Ports: []corev1.ContainerPort{{
					Name:          "syslog-port",
					ContainerPort: 24903,
				}, {
					Name:          "metrics-port",
					ContainerPort: 6060,
				}},
				Env: []corev1.EnvVar{{
					Name:  "SYSLOG_PORT",
					Value: "24903",
				}, {
					Name:  "METRICS_PORT",
					Value: "6060",
				}, {
					Name:  "MESSAGE",
					Value: "test-log-message",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Error creating LogSink: %v", err)
	}

	logger.Info("Getting cluster nodes")
	nodes, err := clusterNodes(clients.kubeClient)
	if err != nil {
		t.Fatalf("Error getting the cluster nodes")
	}

	logger.Info("Waiting for all fluentbit pods to be running")
	fluentState := func(ps *corev1.PodList) (bool, error) {
		var runningCount int
		for _, p := range ps.Items {
			if p.Labels["app"] == "fluent-bit-ds" {
				if p.Status.Phase == corev1.PodRunning {
					runningCount++
				}
			}
		}
		return runningCount == len(nodes.Items), nil
	}
	err = test.WaitForPodListState(
		clients.kubeClient,
		fluentState,
		prefix+"fluent",
		"knative-observability",
	)
	if err != nil {
		t.Fatalf("Error waiting for fluent-bit to be running: %v", err)
	}

	logger.Info("Waiting for syslog receiver to be running")
	syslogState := func(ps *corev1.PodList) (bool, error) {
		for _, p := range ps.Items {
			if p.Labels["app"] == prefix+"syslog-receiver" && p.Status.Phase == corev1.PodRunning {
				return true, nil
			}
		}
		return false, nil
	}
	err = test.WaitForPodListState(
		clients.kubeClient,
		syslogState,
		prefix+"syslog-receiver",
		observabilityTestNamespace,
	)
	if err != nil {
		t.Fatalf("Error waiting for syslog-receiver to be running: %v", err)
	}

	logger.Info("Emitting logs")
	_, err = clients.kubeClient.Kube.Batch().Jobs(observabilityTestNamespace).Create(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "log-emitter",
			Labels: map[string]string{
				"app": prefix + "log-emitter",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": prefix + "log-emitter",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "log-emitter",
						Image: "ubuntu",
						Command: []string{
							"bash",
							"-c",
							"for _ in {1..10}; do echo test-log-message; sleep 0.5; done",
						},
					}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Error creating log-emitter: %v", err)
	}

	logger.Info("Waiting for log-emitter job to be completed")
	logEmitterState := func(ps *corev1.PodList) (bool, error) {
		for _, p := range ps.Items {
			if p.Labels["app"] == prefix+"log-emitter" && p.Status.Phase == corev1.PodSucceeded {
				return true, nil
			}
		}
		return false, nil
	}
	err = test.WaitForPodListState(
		clients.kubeClient,
		logEmitterState,
		prefix+"log-emitter",
		observabilityTestNamespace,
	)
	if err != nil {
		t.Fatalf("Error waiting for log-emitter to be completed: %v", err)
	}

	logger.Info("Get the count for number of logs received")
	_, err = clients.kubeClient.Kube.Batch().Jobs(observabilityTestNamespace).Create(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "log-observer",
			Labels: map[string]string{
				"app": prefix + "log-observer",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": prefix + "log-observer",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "log-observer",
						Image: "oratos/ci-base",
						Command: []string{
							"bash",
							"-c",
							"for _ in {1..10}; do LOG_COUNT=$(curl -s log-sink-syslog-receiver.observability-tests:6060/metrics | jq -r '.namespaced.\"observability-tests\"'); echo \"Logs Received: $LOG_COUNT\"; sleep 1; done",
						},
					}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Error creating log-observer: %v", err)
	}

	logger.Info("Waiting for log-observer job to be completed")
	var logObserverPodName string
	logObserverState := func(ps *corev1.PodList) (bool, error) {
		for _, p := range ps.Items {
			if p.Labels["app"] == prefix+"log-observer" && p.Status.Phase == corev1.PodSucceeded {
				logObserverPodName = p.Name
				return true, nil
			}
		}
		return false, nil
	}
	err = test.WaitForPodListState(
		clients.kubeClient,
		logObserverState,
		prefix+"log-observer",
		observabilityTestNamespace,
	)
	if err != nil {
		t.Fatalf("Error waiting for log-observer to be completed: %v", err)
	}

	req := clients.kubeClient.Kube.Core().Pods(observabilityTestNamespace).GetLogs(
		logObserverPodName,
		&corev1.PodLogOptions{},
	)

	b, err := req.Do().Raw()
	if err != nil {
		t.Fatalf("Error reading logs from the log-observer: %v", err)
	}

	if !strings.Contains(string(b), "Logs Received: 10") {
		t.Fatalf("Received log count is not 10: %s\n", string(b))
	}

}
