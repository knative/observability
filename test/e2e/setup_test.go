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
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	observabilityv1alpha1 "github.com/knative/observability/pkg/client/clientset/versioned/typed/sink/v1alpha1"
	"github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/reference"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"

	oversioned "github.com/knative/observability/pkg/client/clientset/versioned"
)

const (
	observabilityTestNamespace = "observability-tests"
	crosstalkTestNamespace     = "observability-tests-crosstalk"
	syslogReceiverSuffix       = "syslog-receiver"
	serviceAccountName         = "service-account"
	podSecurityPolicyName      = "pod-security-policy"
)

type ReceiverMetrics struct {
	Namespaced        map[string]int `json:"namespaced"`
	WebhookNamespaced map[string]int `json:"webhookNamespaced"`
	Cluster           int            `json:"cluster"`
}

var testRunPrefix = randString(5)

func randomTestPrefix(prefix string) string {
	return fmt.Sprintf("%s-%s", testRunPrefix, prefix)
}

const letters = "abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// initialize is responsible for setting up and tearing down the testing environment,
// namely the test namespace.
func initialize(t *testing.T, name string) (*clients, *logging.BaseLogger) {
	flag.Parse()
	logging.InitializeLogger(test.Flags.LogVerbose)
	logger := logging.GetContextLogger("TestSetup")
	flag.Set("alsologtostderr", "true")
	if test.Flags.EmitMetrics {
		logging.InitializeMetricExporter()
	}

	clients := setup(logger)
	test.CleanupOnInterrupt(func() {
		teardownNamespaces(clients, logger)
	}, logger)
        return clients, logging.GetContextLogger(name)
}

type spdyDialer struct {
	RoundTripper http.RoundTripper
	Upgrader     spdy.Upgrader
	Host         string
}

type clients struct {
	restCfg    *rest.Config
	kubeClient *test.KubeClient
	sinkClient observabilityv1alpha1.ObservabilityV1alpha1Interface
	spdyDialer spdyDialer
}

func teardownNamespace(clients *clients, logger *logging.BaseLogger, namespace string) {
	logger.Infof("Deleting namespace %q", namespace)

	err := clients.kubeClient.Kube.CoreV1().Namespaces().Delete(
		namespace,
		&metav1.DeleteOptions{},
	)
	if err != nil && !kuberrors.IsNotFound(err) {
		logger.Fatalf("Error deleting namespace %q: %v", namespace, err)
	}
}

func teardownNamespaces(clients *clients, logger *logging.BaseLogger) {
	teardownNamespace(clients, logger, observabilityTestNamespace)
	err := waitForNamespaceCleanup(observabilityTestNamespace, clients, logger)
	if err != nil {
		logger.Fatalf("Failed to clean up existing namespace %q", observabilityTestNamespace)
	}
	teardownNamespace(clients, logger, crosstalkTestNamespace)
	err = waitForNamespaceCleanup(crosstalkTestNamespace, clients, logger)
	if err != nil {
		logger.Fatalf("Failed to clean up existing namespace %q", crosstalkTestNamespace)
	}
}

func waitForNamespaceCleanup(ns string, clients *clients, logger *logging.BaseLogger) error {
	for i := 0; i < 300; i++ {
		namespaces, err := clients.kubeClient.Kube.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			logger.Infof("Failed to get namespaces: %s", err)
		}

		var present bool
		for _, namespace := range namespaces.Items {
			if namespace.Name == ns {
				present = true
			}
		}

		if !present {
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("namespace %q still exists", ns)
}

func clusterNodes(client *test.KubeClient) (*corev1.NodeList, error) {
	return client.Kube.CoreV1().Nodes().List(metav1.ListOptions{})
}

func setup(logger *logging.BaseLogger) *clients {
	clients, err := newClients()
	if err != nil {
		logger.Fatalf("Error creating newClients: %v", err)
	}

	// Cleanup before run
	teardownNamespaces(clients, logger)

	createNamespace(clients, logger, observabilityTestNamespace)
	createNamespace(clients, logger, crosstalkTestNamespace)
	createPodSecurityPolicy(clients, logger)
	return clients
}

func createPodSecurityPolicy(clients *clients, logger *logging.BaseLogger) {
	logger.Infof("Creating pod security policy")
	_, err := clients.kubeClient.Kube.PolicyV1beta1().PodSecurityPolicies().Create(&policyv1.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: podSecurityPolicyName,
		},
		Spec: policyv1.PodSecurityPolicySpec{
			Privileged: false,
			SELinux: policyv1.SELinuxStrategyOptions{
				Rule: policyv1.SELinuxStrategyRunAsAny,
			},
			SupplementalGroups: policyv1.SupplementalGroupsStrategyOptions{
				Rule: policyv1.SupplementalGroupsStrategyRunAsAny,
			},
			RunAsUser: policyv1.RunAsUserStrategyOptions{
				Rule: policyv1.RunAsUserStrategyRunAsAny,
			},
			FSGroup: policyv1.FSGroupStrategyOptions{
				Rule: policyv1.FSGroupStrategyRunAsAny,
			},
			Volumes: []policyv1.FSType{
				"*",
			},
		},
	})

	if err != nil {
		if !kuberrors.IsAlreadyExists(err) {

			logger.Fatalf("Error creating pod security policy: %v", err)
		}

		logger.Infof("Pod Security Policy already exists")
	}

	logger.Infof("Created pod security policy")
	logger.Infof("Creating Roles and Bindings and Service Accounts")
	for _, namespace := range []string{observabilityTestNamespace, crosstalkTestNamespace} {

		_, err = clients.kubeClient.Kube.CoreV1().ServiceAccounts(namespace).Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		})
		if err != nil {
			if !kuberrors.IsAlreadyExists(err) {
				logger.Fatalf("Error creating Namespace: %v", err)
			}

			logger.Infof("Namespace already exists")
		}

		roleName := "role-" + namespace

		_, err = clients.kubeClient.Kube.RbacV1().Roles(namespace).Create(
			&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: namespace,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"policy"},
						Resources:     []string{"podsecuritypolicies"},
						ResourceNames: []string{podSecurityPolicyName},
						Verbs:         []string{"use"},
					},
				},
			},
		)

		if err != nil {
			if !kuberrors.IsAlreadyExists(err) {
				logger.Fatalf("Error creating Role: %v", err)
			}

			logger.Infof("Role already exists")
		}
		_, err = clients.kubeClient.Kube.RbacV1().RoleBindings(namespace).Create(
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role-binding-" + namespace,
					Namespace: namespace,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:     "Group",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "system:authenticated",
					},
					{
						Kind:     "Group",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     "system:serviceaccounts",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind:     "Role",
					Name:     roleName,
					APIGroup: "rbac.authorization.k8s.io",
				},
			},
		)
		if err != nil {
			if !kuberrors.IsAlreadyExists(err) {

				logger.Fatalf("Error creating Role Binding: %v", err)
			}

			logger.Infof("Role Binding already exists")
		}
	}
}

func createNamespace(clients *clients, logger *logging.BaseLogger, namespace string) {
	logger.Infof("Creating namespace %q", namespace)
	// Ensure the test namespace exists, by trying to create it and ignoring
	// already-exists errors.
	_, err := clients.kubeClient.Kube.CoreV1().Namespaces().Create(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		},
	)

	if err != nil {
		if kuberrors.IsAlreadyExists(err) {
			logger.Infof("Namespace %q already exists", namespace)

			return
		}

		logger.Fatalf("Error creating namespace %q: %v", namespace, err)
	}

	logger.Infof("Created namespace %q", namespace)
}

func newClients() (*clients, error) {
	configPath := test.Flags.Kubeconfig
	clusterName := test.Flags.Cluster

	overrides := clientcmd.ConfigOverrides{}
	// Override the cluster name if provided.
	if clusterName != "" {
		overrides.Context.Cluster = clusterName
	}

	restCfg, err := test.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}

	rt, up, err := spdy.RoundTripperFor(restCfg)
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: configPath,
		},
		&overrides,
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	sc, err := oversioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &clients{
		restCfg: restCfg,
		kubeClient: &test.KubeClient{
			Kube: kubeClient,
		},
		sinkClient: sc.ObservabilityV1alpha1(),
		spdyDialer: spdyDialer{
			RoundTripper: rt,
			Upgrader:     up,
			Host:         restCfg.Host,
		},
	}, nil
}

func assertErr(t *testing.T, msg string, err error) {
	if err != nil {
		t.Fatalf(msg, err)
	}
}

func createSyslogReceiver(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	kc *test.KubeClient,
	namespace string,
) {
	logger.Info("Creating the service for the syslog receiver")
	_, err := kc.Kube.CoreV1().Services(namespace).Create(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + syslogReceiverSuffix,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "syslog",
					Port: 24903,
				},
				{
					Name: "metrics",
					Port: 6060,
				},
				{
					Name: "http",
					Port: 7070,
				},
			},
			Selector: map[string]string{
				"app": prefix + syslogReceiverSuffix,
			},
		},
	})
	assertErr(t, "Error creating Syslog Receiver Service: %v", err)

	logger.Info("Creating the pod for the syslog receiver")
	_, err = kc.Kube.CoreV1().Pods(namespace).Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + syslogReceiverSuffix,
			Labels: map[string]string{
				"app":      prefix + syslogReceiverSuffix,
				"test-pod": syslogReceiverSuffix,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccountName,
			Containers: []corev1.Container{{
				Name:            syslogReceiverSuffix,
				Image:           "oratos/crosstalk-receiver:v0.5",
				ImagePullPolicy: corev1.PullAlways,
				Ports: []corev1.ContainerPort{
					{
						Name:          "syslog-port",
						ContainerPort: 24903,
					},
					{
						Name:          "metrics-port",
						ContainerPort: 6060,
					},
					{
						Name:          "http-port",
						ContainerPort: 7070,
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "SYSLOG_PORT",
						Value: "24903",
					},
					{
						Name:  "METRICS_PORT",
						Value: "6060",
					},
					{
						Name:  "HTTP_PORT",
						Value: "7070",
					},
					{
						Name:  "MESSAGE",
						Value: prefix + "test-log-message",
					},
				},
			}},
		},
	})
	assertErr(t, "Error creating Syslog Receiver: %v", err)

	logger.Info("Waiting for syslog receiver to be running")
	syslogState := func(ps *corev1.PodList) (bool, error) {
		for _, p := range ps.Items {
			if p.Labels["app"] == prefix+syslogReceiverSuffix && p.Status.Phase == corev1.PodRunning {
				return true, nil
			}
		}
		return false, nil
	}
	err = test.WaitForPodListState(
		kc,
		syslogState,
		prefix+syslogReceiverSuffix,
		namespace,
	)
	assertErr(t, "Error waiting for syslog-receiver to be running: %v", err)
}

func waitForTelegrafToBeReady(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	label string,
	namespace string,
	kc *test.KubeClient,
) {
	logger.Info("Giving metric-sink-controller time to delete telegraf pods")
	time.Sleep(5 * time.Second)

	logger.Info("Waiting for all telegraf pods to be ready")
	telegrafState := func(ps *corev1.PodList) (bool, error) {
		for _, p := range ps.Items {
			if p.Labels["app"] == label && ready(p) {
				return true, nil
			}
		}
		return false, nil
	}
	err := test.WaitForPodListState(
		kc,
		telegrafState,
		prefix+"telegraf",
		namespace,
	)
	assertErr(t, "Error waiting for telegraf to be ready: %v", err)
}

func waitForFluentBitToBeReady(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	kc *test.KubeClient,
) {
	logger.Info("Giving sink-controller time to delete fluentbit pods")
	time.Sleep(5 * time.Second)

	logger.Info("Getting cluster nodes")
	nodes, err := clusterNodes(kc)
	assertErr(t, "Error getting the cluster nodes: %v", err)

	logger.Info("Waiting for all fluentbit pods to be ready")
	fluentState := func(ps *corev1.PodList) (bool, error) {
		var readyCount int
		for _, p := range ps.Items {
			if p.Labels["app"] == "fluent-bit" && ready(p) {
				readyCount++
			}
		}
		return readyCount == len(nodes.Items), nil
	}
	err = test.WaitForPodListState(
		kc,
		fluentState,
		prefix+"fluent",
		"knative-observability",
	)
	assertErr(t, "Error waiting for fluent-bit to be ready: %v", err)
}

func ready(p corev1.Pod) bool {
	if len(p.Status.ContainerStatuses) == 0 {
		return false
	}
	for _, s := range p.Status.ContainerStatuses {
		if !s.Ready {
			return false
		}
	}
	return true
}

func getPodName(
	t *testing.T,
	kc *test.KubeClient,
	namespace string,
	podLabel string,
) string {

	podList, err := kc.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: podLabel,
	})
	assertErr(t, "Failed to get pod", err)

	for _, p := range podList.Items {
		if ready(p) {
			return p.Name
		}
	}
	t.Fatalf("Could not find ready pod matching label %s", podLabel)
	return ""
}

type TelegrafMetric struct {
	Fields ValueField `json:"fields"`
	Name   string     `json:"name"`
}

type ValueField struct {
	Value int `json:"value"`
}

func assertTelegrafOutputtedData(
	t *testing.T,
	logger *logging.BaseLogger,
	label string,
	namespace string,
	kc *test.KubeClient,
	restCfg *rest.Config,
) {
	var err error
	waitTime := 20
	for timeWaited := 0; waitTime >= timeWaited; timeWaited++ {
		logger.Infof("Checking output of telegraf")
		err = checkTelegrafOutputtedData(t, label, namespace, kc, restCfg)
		if err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("Error looking for telegraf output: %s\n", err)
}

func checkTelegrafOutputtedData(
	t *testing.T,
	label string,
	namespace string,
	kc *test.KubeClient,
	restCfg *rest.Config,
) error {
	podName := getPodName(t, kc, namespace, label)
	req := kc.Kube.
		CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "telegraf",
			Command:   []string{"cat", "/tmp/test"},
			Stdin:     false,
			Stdout:    true,
			Stderr:    false,
			TTY:       false,
		}, scheme.ParameterCodec)
	re, err := remotecommand.NewSPDYExecutor(restCfg, "POST", req.URL())
	if err != nil {
		return err
	}

	var outBuf bytes.Buffer
	err = re.Stream(remotecommand.StreamOptions{
		Stdout: &outBuf,
	})

	if err != nil {
		return err
	}

	dec := json.NewDecoder(strings.NewReader(outBuf.String()))

	for {
		var m TelegrafMetric
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("Unable to decode Telegraf Metric: %s", err.Error())
		}

		if m.Name == "test" {
			if m.Fields.Value != 5 {
				t.Fatalf("Value for metric bad, expected:%d Got:%d\n", 5, m.Fields.Value)
			}
			return nil
		}
	}

	return errors.New("failed to find metric")
}

func assertOnCrosstalk(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	clients *clients,
	namespace string,
	assert func(ReceiverMetrics) error,
) {
	fports, cancel, err := portForward(
		namespace,
		prefix+syslogReceiverSuffix,
		[]string{"6060:6060"},
		clients,
		logger,
	)
	assertErr(t, "Failed to open port-forward: %s", err)
	defer cancel()

	if len(fports) != 1 {
		t.Fatalf("Unable to get the forwarded ports")
	}

	client := &http.Client{
		Transport: clients.spdyDialer.RoundTripper,
		Timeout:   time.Second * 2,
	}

	var metrics ReceiverMetrics
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	var cause error

	for {
		select {
		case <-tick.C:
			metrics, err = getMetrics(client)
			assertErr(t, "Failed to get metrics %s", err)

			if cause = assert(metrics); cause == nil {
				return
			}
		case <-timeout.C:
			t.Fatalf("Expecting assertation to succeed, got %#v %s", metrics, cause)
		}
	}
}

func getMetrics(client *http.Client) (ReceiverMetrics, error) {
	resp, err := client.Get("http://127.0.0.1:6060/metrics")
	if err != nil {
		return ReceiverMetrics{}, fmt.Errorf("Unable to GET /metrics: %s", err)
	}
	defer resp.Body.Close()

	var rm ReceiverMetrics
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ReceiverMetrics{}, fmt.Errorf("Unable to read response body: %s", err)
	}

	err = json.Unmarshal(body, &rm)
	if err != nil {
		return ReceiverMetrics{}, fmt.Errorf("Unable to unmarshal response body: %s", err)
	}

	return rm, nil
}

func portForward(
	ns string,
	appName string,
	ports []string,
	clients *clients,
	logger *logging.BaseLogger,
) ([]portforward.ForwardedPort, func(), error) {
	pods, err := clients.kubeClient.Kube.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: "app=" + appName,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("Unable to get syslog receiver pod list: %s", err)
	}

	if len(pods.Items) != 1 {
		return nil, nil, errors.New("Unable to get the syslog receiver pod")
	}

	syslogReceiverPodName := pods.Items[0].Name

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", ns, syslogReceiverPodName)
	hostIP := strings.TrimPrefix(clients.spdyDialer.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}
	logger.Infof("Server URL: %s", serverURL.String())

	httpClient := &http.Client{
		Transport: clients.spdyDialer.RoundTripper,
		Timeout:   time.Second * 2,
	}
	dialer := spdy.NewDialer(clients.spdyDialer.Upgrader, httpClient, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	errOut := new(bytes.Buffer)

	// Would prefer to have random local port 0:6060, but forwarder.GetPorts()
	// has a bug. See here for details:
	// https://github.com/kubernetes/kubernetes/issues/69052
	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, ioutil.Discard, errOut)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to create new port forwarder: %s", err)
	}

	logger.Info("Forwarding ports to syslog-receiver 6060")
	go func() {
		err := forwarder.ForwardPorts()
		if err != nil {
			logger.Errorf("Port forwarding failed: %s", err)
		}
	}()

	select {
	case <-readyChan:
		logger.Info("Port forwarding ready")
		if len(errOut.String()) != 0 {
			close(stopChan)
			return nil, nil, errors.New(errOut.String())
		}
	case <-time.After(5 * time.Second):
		close(stopChan)
		return nil, nil, errors.New("Didn't port forward within timeout")
	}

	var fports []portforward.ForwardedPort
	for i := 0; i < 5; i++ {
		fports, err = forwarder.GetPorts()
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}

	cancelFn := func() {
		logger.Info("Closing forwarded ports")
		close(stopChan)
	}

	return fports, cancelFn, nil
}

func emitLogs(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	kc *test.KubeClient,
	namespace string,
) {
	logger.Info("Emitting logs")
	_, err := kc.Kube.BatchV1().Jobs(namespace).Create(&batchv1.Job{
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
					ServiceAccountName: serviceAccountName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "log-emitter",
						Image: "ubuntu:xenial",
						Command: []string{
							"bash",
							"-c",
							fmt.Sprintf("for _ in {1..10}; do echo %stest-log-message; sleep 0.5; done", prefix),
						},
					}},
				},
			},
		},
	})
	assertErr(t, "Error creating log-emitter: %v", err)

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
		kc,
		logEmitterState,
		prefix+"log-emitter",
		namespace,
	)
	assertErr(t, "Error waiting for log-emitter to be completed: %v", err)
}

func emitEvents(
	t *testing.T,
	logger *logging.BaseLogger,
	prefix string,
	kc *test.KubeClient,
	namespace string,
) {
	logger.Info("Creating Job that can be referenced for events")
	name := prefix + "job"
	job, err := kc.Kube.BatchV1().Jobs(namespace).Create(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": prefix + "job",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": prefix + "job",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  name,
						Image: "ubuntu:xenial",
						Command: []string{
							"bash",
							"-c",
							"sleep 0.5",
						},
					}},
				},
			},
		},
	})
	assertErr(t, "Error creating job: %v", err)

	ref, err := reference.GetReference(scheme.Scheme, runtime.Object(job))
	assertErr(t, "Error getting reference: %v", err)

	eventCreator := kc.Kube.CoreV1().Events(namespace)
	time.Sleep(3 * time.Second)
	for i := 0; i < 10; i++ {
		event := &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", ref.UID, i),
				Namespace: namespace,
			},
			InvolvedObject: *ref,
			Message:        fmt.Sprintf("%stest-log-message", prefix),
			Reason:         "reason",
			Type:           corev1.EventTypeNormal,
		}
		_, err = eventCreator.Create(event)
		assertErr(t, "Error creating event: %v", err)
	}
}
