package metric_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	typedrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"

	sinkv1alpha1 "github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/metric"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetricSink(t *testing.T) {
	t.Run("it sets defaults for Kind and APIVersion if missing on object so it can populate OwnerReferences correctly", func(t *testing.T) {
		var (
			createConfigMapCalled bool
			createReceivedCM      v1.ConfigMap
		)
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					createConfigMapCalled = true
					createReceivedCM = *cm
					return cm, nil
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
			spyPodDeleter: spyPodDeleter{},
		}

		var (
			createDeploymentCalled bool
			receivedDeployment     appsv1.Deployment
		)
		spyExtensionsClient := &spyAppsV1Client{
			spyTelegrafDeploymentCUDer: spyTelegrafDeploymentCUDer{
				createFunc: func(d *appsv1.Deployment) (*appsv1.Deployment, error) {
					createDeploymentCalled = true
					receivedDeployment = *d
					return d, nil
				},
				updateFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		var (
			createRoleCalled        bool
			createRoleBindingCalled bool
			receivedRole            rbacv1.Role
			receivedRoleBinding     rbacv1.RoleBinding
		)
		spyRBACClient := &spyRBACV1Client{
			spyRoleCUDer: spyRoleCUDer{
				createFunc: func(r *rbacv1.Role) (*rbacv1.Role, error) {
					createRoleCalled = true
					receivedRole = *r
					return r, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
			spyRoleBindingCUDer: spyRoleBindingCUDer{
				createFunc: func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					createRoleBindingCalled = true
					receivedRoleBinding = *rb
					return rb, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, spyRBACClient)
		d := &sinkv1alpha1.MetricSink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
				UID:       "some-random-uid",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{
					{
						"type": "cpu",
					},
				},
				Outputs: []sinkv1alpha1.MetricSinkMap{
					{
						"type":   "datadog",
						"apikey": "some-key",
					},
				},
			},
		}

		c.OnAdd(d)

		metricSinkConf := `[global_tags]
  cluster_name = "test-cluster-name"

[inputs]

  [[inputs.cpu]]

  [[inputs.prometheus]]
    monitor_kubernetes_pods = true
    monitor_kubernetes_pods_namespace = "test-namespace"

[outputs]

  [[outputs.datadog]]
    apikey = "some-key"
`

		expectedConfigMap := v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "observability.knative.dev/v1alpha1",
					Kind:       "MetricSink",
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Data: map[string]string{
				"metric-sinks.conf": metricSinkConf,
				"telegraf.conf":     metric.DefaultTelegrafConf,
			},
		}

		var r int32 = 1
		expectedDeployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "observability.knative.dev/v1alpha1",
					Kind:       "MetricSink",
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "telegraf-test-metric-sink"},
				},
				Replicas: &r,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "telegraf-test-metric-sink",
						},
					},
					Spec: v1.PodSpec{
						Volumes: []v1.Volume{{
							Name: "telegraf-config",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "telegraf-test-metric-sink",
									},
								},
							},
						}},
						Containers: []v1.Container{{
							Name:    "telegraf",
							Image:   "telegraf:" + metric.TelegrafImageVersion,
							Command: []string{"telegraf", "--config-directory", "/etc/telegraf"},
							VolumeMounts: []v1.VolumeMount{{
								Name:      "telegraf-config",
								MountPath: "/etc/telegraf",
							}},
							ImagePullPolicy: "IfNotPresent",
						}},
					},
				},
			},
		}

		expectedRole := rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "observability.knative.dev/v1alpha1",
					Kind:       "MetricSink",
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"use"},
					APIGroups:     []string{"extensions"},
					Resources:     []string{"podsecuritypolicies"},
					ResourceNames: []string{"telegraf"},
				},
				{
					Verbs:     []string{"get", "list", "watch"},
					APIGroups: []string{""},
					Resources: []string{"pods"},
				},
			},
		}

		expectedRoleBinding := rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "observability.knative.dev/v1alpha1",
					Kind:       "MetricSink",
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: "test-namespace",
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "telegraf-test-metric-sink",
			},
		}

		if !createConfigMapCalled {
			t.Fatalf("ConfigMap Create not called")
		}
		if diff := cmp.Diff(createReceivedCM, expectedConfigMap); diff != "" {
			t.Fatalf("ConfigMap does not equal expected (-want +got): %v", diff)
		}
		if !createDeploymentCalled {
			t.Fatalf("Deployment Create not called")
		}
		if diff := cmp.Diff(receivedDeployment, expectedDeployment); diff != "" {
			t.Fatalf("Deployment does not equal expected (-want +got): %v", diff)
		}
		if !createRoleCalled {
			t.Fatalf("Role Create not called")
		}
		if diff := cmp.Diff(receivedRole, expectedRole); diff != "" {
			t.Fatalf("Role does not equal expected (-want +got): %v", diff)
		}
		if !createRoleBindingCalled {
			t.Fatalf("RoleBinding Create not called")
		}
		if diff := cmp.Diff(receivedRoleBinding, expectedRoleBinding); diff != "" {
			t.Fatalf("RoleBinding does not equal expected (-want +got): %v", diff)
		}
	})

	t.Run("it creates a telegraf config map, deployment, roles, and bindings in the specified namespace", func(t *testing.T) {
		var (
			createConfigMapCalled bool
			createReceivedCM      v1.ConfigMap
		)
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					createConfigMapCalled = true
					createReceivedCM = *cm
					return cm, nil
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
			spyPodDeleter: spyPodDeleter{},
		}

		var (
			createDeploymentCalled bool
			receivedDeployment     appsv1.Deployment
		)
		spyExtensionsClient := &spyAppsV1Client{
			spyTelegrafDeploymentCUDer: spyTelegrafDeploymentCUDer{
				createFunc: func(d *appsv1.Deployment) (*appsv1.Deployment, error) {
					createDeploymentCalled = true
					receivedDeployment = *d
					return d, nil
				},
				updateFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		var (
			createRoleCalled        bool
			createRoleBindingCalled bool
			receivedRole            rbacv1.Role
			receivedRoleBinding     rbacv1.RoleBinding
		)
		spyRBACClient := &spyRBACV1Client{
			spyRoleCUDer: spyRoleCUDer{
				createFunc: func(r *rbacv1.Role) (*rbacv1.Role, error) {
					createRoleCalled = true
					receivedRole = *r
					return r, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
			spyRoleBindingCUDer: spyRoleBindingCUDer{
				createFunc: func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					createRoleBindingCalled = true
					receivedRoleBinding = *rb
					return rb, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, spyRBACClient)
		d := &sinkv1alpha1.MetricSink{
			TypeMeta: metav1.TypeMeta{
				Kind:       "MetricSink",
				APIVersion: "some-api/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
				UID:       "some-random-uid",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{
					{
						"type": "cpu",
					},
				},
				Outputs: []sinkv1alpha1.MetricSinkMap{
					{
						"type":   "datadog",
						"apikey": "some-key",
					},
				},
			},
		}

		c.OnAdd(d)

		metricSinkConf := `[global_tags]
  cluster_name = "test-cluster-name"

[inputs]

  [[inputs.cpu]]

  [[inputs.prometheus]]
    monitor_kubernetes_pods = true
    monitor_kubernetes_pods_namespace = "test-namespace"

[outputs]

  [[outputs.datadog]]
    apikey = "some-key"
`

		expectedConfigMap := v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: d.APIVersion,
					Kind:       d.Kind,
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Data: map[string]string{
				"metric-sinks.conf": metricSinkConf,
				"telegraf.conf":     metric.DefaultTelegrafConf,
			},
		}

		var r int32 = 1
		expectedDeployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: d.APIVersion,
					Kind:       d.Kind,
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "telegraf-test-metric-sink"},
				},
				Replicas: &r,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "telegraf-test-metric-sink",
						},
					},
					Spec: v1.PodSpec{
						Volumes: []v1.Volume{{
							Name: "telegraf-config",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "telegraf-test-metric-sink",
									},
								},
							},
						}},
						Containers: []v1.Container{{
							Name:    "telegraf",
							Image:   "telegraf:" + metric.TelegrafImageVersion,
							Command: []string{"telegraf", "--config-directory", "/etc/telegraf"},
							VolumeMounts: []v1.VolumeMount{{
								Name:      "telegraf-config",
								MountPath: "/etc/telegraf",
							}},
							ImagePullPolicy: "IfNotPresent",
						}},
					},
				},
			},
		}

		expectedRole := rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: d.APIVersion,
					Kind:       d.Kind,
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:         []string{"use"},
					APIGroups:     []string{"extensions"},
					Resources:     []string{"podsecuritypolicies"},
					ResourceNames: []string{"telegraf"},
				},
				{
					Verbs:     []string{"get", "list", "watch"},
					APIGroups: []string{""},
					Resources: []string{"pods"},
				},
			},
		}

		expectedRoleBinding := rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: d.APIVersion,
					Kind:       d.Kind,
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: "test-namespace",
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "telegraf-test-metric-sink",
			},
		}

		if !createConfigMapCalled {
			t.Fatalf("ConfigMap Create not called")
		}
		if diff := cmp.Diff(createReceivedCM, expectedConfigMap); diff != "" {
			t.Fatalf("ConfigMap does not equal expected (-want +got): %v", diff)
		}
		if !createDeploymentCalled {
			t.Fatalf("Deployment Create not called")
		}
		if diff := cmp.Diff(receivedDeployment, expectedDeployment); diff != "" {
			t.Fatalf("Deployment does not equal expected (-want +got): %v", diff)
		}
		if !createRoleCalled {
			t.Fatalf("Role Create not called")
		}
		if diff := cmp.Diff(receivedRole, expectedRole); diff != "" {
			t.Fatalf("Role does not equal expected (-want +got): %v", diff)
		}
		if !createRoleBindingCalled {
			t.Fatalf("RoleBinding Create not called")
		}
		if diff := cmp.Diff(receivedRoleBinding, expectedRoleBinding); diff != "" {
			t.Fatalf("RoleBinding does not equal expected (-want +got): %v", diff)
		}
	})

	t.Run("it doesn't add cluster_name global tag if it is empty", func(t *testing.T) {
		var createCalled bool
		var createReceivedCM v1.ConfigMap
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					createCalled = true
					createReceivedCM = *cm
					return cm, nil
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}
		var createDeploymentCalled bool
		spyExtensionsClient := &spyAppsV1Client{
			spyTelegrafDeploymentCUDer: spyTelegrafDeploymentCUDer{
				createFunc: func(d *appsv1.Deployment) (*appsv1.Deployment, error) {
					createDeploymentCalled = true
					return d, nil
				},
				updateFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		spyRBACClient := &spyRBACV1Client{
			spyRoleCUDer: spyRoleCUDer{
				createFunc: func(r *rbacv1.Role) (*rbacv1.Role, error) {
					return r, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
			spyRoleBindingCUDer: spyRoleBindingCUDer{
				createFunc: func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return rb, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		c := metric.NewController("", spyCoreClient, spyExtensionsClient, spyRBACClient)
		d := &sinkv1alpha1.MetricSink{
			TypeMeta: metav1.TypeMeta{
				Kind:       "MetricSink",
				APIVersion: "myapi/v1alpha12",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
				UID:       "metric-sink-uid",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{
					{
						"type": "cpu",
					},
				},
				Outputs: []sinkv1alpha1.MetricSinkMap{
					{
						"type":   "datadog",
						"apikey": "some-key",
					},
				},
			},
		}

		c.OnAdd(d)

		metricSinkConf := `[inputs]

  [[inputs.cpu]]

  [[inputs.prometheus]]
    monitor_kubernetes_pods = true
    monitor_kubernetes_pods_namespace = "test-namespace"

[outputs]

  [[outputs.datadog]]
    apikey = "some-key"
`

		expectedConfigMap := v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: d.APIVersion,
					Kind:       d.Kind,
					Name:       d.Name,
					UID:        d.UID,
				}},
			},
			Data: map[string]string{
				"metric-sinks.conf": metricSinkConf,
				"telegraf.conf":     metric.DefaultTelegrafConf,
			},
		}

		if !createCalled {
			t.Fatalf("ConfigMap Create not called")
		}
		if diff := cmp.Diff(createReceivedCM, expectedConfigMap); diff != "" {
			t.Fatalf("ConfigMap does not equal expected (-want +got): %v", diff)
		}
		if !createDeploymentCalled {
			t.Fatalf("Deployment Create not called")
		}
	})

	t.Run("it updates telegraf config map and deletes the pod in the specified namespace", func(t *testing.T) {
		var updateCalled bool
		var updateReceivedCM v1.ConfigMap
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				updateFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					updateCalled = true
					updateReceivedCM = *cm
					return nil, nil
				},
				createFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}
		spyExtensionsClient := &spyAppsV1Client{}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, nil)

		oms := &sinkv1alpha1.MetricSink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{{
					"type": "cpu",
				}},
				Outputs: []sinkv1alpha1.MetricSinkMap{{
					"type":   "datadog",
					"apikey": "some-key",
				}},
			},
		}

		nms := &sinkv1alpha1.MetricSink{}
		*nms = *oms
		nms.Spec.Inputs = append(nms.Spec.Inputs, sinkv1alpha1.MetricSinkMap{"type": "mem"})
		c.OnUpdate(oms, nms)

		metricSinkConf := `[global_tags]
  cluster_name = "test-cluster-name"

[inputs]

  [[inputs.cpu]]

  [[inputs.mem]]

  [[inputs.prometheus]]
    monitor_kubernetes_pods = true
    monitor_kubernetes_pods_namespace = "test-namespace"

[outputs]

  [[outputs.datadog]]
    apikey = "some-key"
`
		expectedConfigMap := v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telegraf-test-metric-sink",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"app": "telegraf-test-metric-sink",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: nms.APIVersion,
					Kind:       nms.Kind,
					Name:       nms.Name,
					UID:        nms.UID,
				}},
			},
			Data: map[string]string{
				"metric-sinks.conf": metricSinkConf,
				"telegraf.conf":     metric.DefaultTelegrafConf,
			},
		}
		if !updateCalled {
			t.Fatalf("ConfigMap update not called")
		}

		if diff := cmp.Diff(updateReceivedCM, expectedConfigMap); diff != "" {
			t.Fatalf("ConfigMap does not equal expected (-want +got): %v", diff)
		}
		if !spyCoreClient.spyPodDeleter.called {
			t.Fatal("Expected pod deleter to be called")
		}
		expectedListOptions := metav1.ListOptions{
			LabelSelector: "app=telegraf-test-metric-sink",
		}
		if spyCoreClient.spyPodDeleter.receivedListOptions != expectedListOptions {
			t.Fatalf("\nExpected: %+v\nReceived: %+v\n", expectedListOptions, spyCoreClient.spyPodDeleter.receivedListOptions)
		}
	})

	t.Run("it deletes the configmap, deployment, role, rolebinding, and serviceaccount when metric sink is deleted", func(t *testing.T) {
		var (
			cmDeleteCalled          bool
			cmDeleted               string
			tdDeleteCalled          bool
			tdDeleted               string
			roleDeleteCalled        bool
			roleDeleted             string
			roleBindingDeleteCalled bool
			roleBindingDeleted      string
		)

		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				deleteFunc: func(name string, opt *metav1.DeleteOptions) error {
					cmDeleteCalled = true
					cmDeleted = name
					return nil
				},
				createFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
			},
		}

		spyExtensionsClient := &spyAppsV1Client{
			spyTelegrafDeploymentCUDer: spyTelegrafDeploymentCUDer{
				createFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				updateFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("should not be called")
					return nil, nil
				},

				deleteFunc: func(name string, options *metav1.DeleteOptions) error {
					tdDeleteCalled = true
					tdDeleted = name
					return nil
				},
			},
		}

		spyRBACClient := &spyRBACV1Client{
			spyRoleCUDer: spyRoleCUDer{
				createFunc: func(*rbacv1.Role) (*rbacv1.Role, error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(r string, _ *metav1.DeleteOptions) error {
					roleDeleteCalled = true
					roleDeleted = r
					return nil
				},
			},
			spyRoleBindingCUDer: spyRoleBindingCUDer{
				createFunc: func(*rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(rb string, _ *metav1.DeleteOptions) error {
					roleBindingDeleteCalled = true
					roleBindingDeleted = rb
					return nil
				},
			},
		}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, spyRBACClient)

		ms := &sinkv1alpha1.MetricSink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{{
					"type": "cpu",
				}},
				Outputs: []sinkv1alpha1.MetricSinkMap{{
					"type":   "datadog",
					"apikey": "some-key",
				}},
			},
		}

		c.OnDelete(ms)

		if !cmDeleteCalled {
			t.Fatal("Expected config map delete to have been called")
		}
		if cmDeleted != "telegraf-test-metric-sink" {
			t.Fatalf("Expected config map delete to have been called with name \"telegraf-test-metric-sink\", \"received %s\"",
				tdDeleted)
		}
		if !tdDeleteCalled {
			t.Fatal("Expected deployment delete to have been called")
		}
		if tdDeleted != "telegraf-test-metric-sink" {
			t.Fatalf("Expected deployment delete to have been called with name \"telegraf-test-metric-sink\", \"received %s\"",
				tdDeleted)
		}
		if !roleDeleteCalled {
			t.Fatal("Expected role delete to have been called")
		}
		if roleDeleted != "telegraf-test-metric-sink" {
			t.Fatalf("Expected role delete to have been called with name \"telegraf-test-metric-sink\", \"received %s\"",
				tdDeleted)
		}
		if !roleBindingDeleteCalled {
			t.Fatal("Expected rolebinding delete to have been called")
		}
		if roleBindingDeleted != "telegraf-test-metric-sink" {
			t.Fatalf("Expected rolebinding delete to have been called with name \"telegraf-test-metric-sink\", \"received %s\"",
				tdDeleted)
		}
	})

	t.Run("it does not create a deployment if it fails to create a config map", func(t *testing.T) {
		var createCalled bool
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					createCalled = true
					return nil, fmt.Errorf("error creating configmap")
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}
		spyExtensionsClient := &spyAppsV1Client{
			spyTelegrafDeploymentCUDer: spyTelegrafDeploymentCUDer{
				createFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("Telegraf deployment should not have been created")
					return nil, nil
				},
				updateFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("Telegraf deployment should not have been updated")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("Telegraf deployment should not have been deleted")
					return nil
				},
			},
		}
		spyRBACClient := &spyRBACV1Client{
			spyRoleCUDer: spyRoleCUDer{
				createFunc: func(*rbacv1.Role) (*rbacv1.Role, error) {
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
			spyRoleBindingCUDer: spyRoleBindingCUDer{
				createFunc: func(*rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, spyRBACClient)

		ms := &sinkv1alpha1.MetricSink{}

		c.OnAdd(ms)

		if !createCalled {
			t.Fatal("Expected config map create to have been called")
		}
	})

	t.Run("it does not delete the telegraf pod if it fails to update the config map", func(t *testing.T) {
		var updateCalled bool
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				updateFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					updateCalled = true
					return nil, fmt.Errorf("error updating configMap")
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}
		spyExtensionsClient := &spyAppsV1Client{}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, nil)

		nms := &sinkv1alpha1.MetricSink{
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{{
					"type": "cpu",
				}},
			},
		}

		oms := &sinkv1alpha1.MetricSink{}
		c.OnUpdate(oms, nms)

		if !updateCalled {
			t.Fatal("Expected config map update to have been called")
		}
		if spyCoreClient.spyPodDeleter.called {
			t.Fatal("Telegraf pods should not have been deleted")
		}
	})

	t.Run("it only updates if there are changes to the spec property", func(t *testing.T) {
		var updateCalled bool
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					return nil, fmt.Errorf("error updating configMap")
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					t.Fatal("should not be called")
					return nil
				},
			},
		}
		spyExtensionsClient := &spyAppsV1Client{}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, nil)

		o := &sinkv1alpha1.MetricSink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{{
					"type": "cpu",
				}},
				Outputs: []sinkv1alpha1.MetricSinkMap{{
					"type":   "datadog",
					"apikey": "some-key",
				}},
			},
		}
		n := o
		n.Labels = map[string]string{"team": "oratos"}

		c.OnUpdate(o, n)

		if updateCalled {
			t.Fatal("Config map should not have been updated")
		}

		if spyCoreClient.spyPodDeleter.called {
			t.Fatal("Telegraf pods should not have been deleted")
		}
	})

	t.Run("it does not delete the telegraf deployment if it fails to delete the config map", func(t *testing.T) {
		var deleteCalled bool
		spyCoreClient := &spyCoreV1Client{
			spyConfigMapCUDer: spyConfigMapCUDer{
				createFunc: func(cm *v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				updateFunc: func(*v1.ConfigMap) (configMap *v1.ConfigMap, e error) {
					t.Fatal("should not be called")
					return nil, nil
				},
				deleteFunc: func(string, *metav1.DeleteOptions) error {
					deleteCalled = true
					return fmt.Errorf("error deleting configMap")
				},
			},
		}

		spyExtensionsClient := &spyAppsV1Client{
			spyTelegrafDeploymentCUDer: spyTelegrafDeploymentCUDer{
				createFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("Telegraf deployment should not have been created")
					return nil, nil
				},
				updateFunc: func(*appsv1.Deployment) (*appsv1.Deployment, error) {
					t.Fatal("Telegraf deployment should not have been updated")
					return nil, nil
				},
				deleteFunc: func(name string, options *metav1.DeleteOptions) error {
					t.Fatal("Telegraf deployment should not have been deleted")
					return nil
				},
			},
		}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, nil)

		o := &sinkv1alpha1.MetricSink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-metric-sink",
				Namespace: "test-namespace",
			},
			Spec: sinkv1alpha1.MetricSinkSpec{
				Inputs: []sinkv1alpha1.MetricSinkMap{{
					"type": "cpu",
				}},
				Outputs: []sinkv1alpha1.MetricSinkMap{{
					"type":   "datadog",
					"apikey": "some-key",
				}},
			},
		}

		c.OnDelete(o)

		if !deleteCalled {
			t.Fatal("Config map delete should have been called")
		}
	})

	t.Run("it should not panic if it is not a metric sink", func(t *testing.T) {
		spyCoreClient := &spyCoreV1Client{}
		spyExtensionsClient := &spyAppsV1Client{}

		c := metric.NewController("test-cluster-name", spyCoreClient, spyExtensionsClient, nil)

		c.OnAdd("")
		c.OnUpdate(nil, nil)
		c.OnDelete(1)
	})
}

type spyCoreV1Client struct {
	spyConfigMapCUDer
	spyPodDeleter
}

func (c *spyCoreV1Client) Pods(namespace string) typedv1.PodInterface {
	return &c.spyPodDeleter
}

func (c *spyCoreV1Client) ConfigMaps(namespace string) typedv1.ConfigMapInterface {
	return &c.spyConfigMapCUDer
}

type spyPodDeleter struct {
	called              bool
	receivedListOptions metav1.ListOptions
}

func (s *spyPodDeleter) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	s.called = true
	s.receivedListOptions = listOptions
	return nil
}

type spyConfigMapCUDer struct {
	createFunc func(cm *v1.ConfigMap) (*v1.ConfigMap, error)
	updateFunc func(cm *v1.ConfigMap) (*v1.ConfigMap, error)
	deleteFunc func(name string, options *metav1.DeleteOptions) error
}

func (s *spyConfigMapCUDer) Create(cm *v1.ConfigMap) (*v1.ConfigMap, error) {
	return s.createFunc(cm)
}

func (s *spyConfigMapCUDer) Update(cm *v1.ConfigMap) (*v1.ConfigMap, error) {
	return s.updateFunc(cm)
}

func (s *spyConfigMapCUDer) Delete(name string, options *metav1.DeleteOptions) error {
	return s.deleteFunc(name, options)
}

type spyAppsV1Client struct {
	spyTelegrafDeploymentCUDer
}

func (s *spyAppsV1Client) Deployments(namespace string) typedappsv1.DeploymentInterface {
	return &s.spyTelegrafDeploymentCUDer
}

type spyTelegrafDeploymentCUDer struct {
	createFunc func(*appsv1.Deployment) (*appsv1.Deployment, error)
	updateFunc func(*appsv1.Deployment) (*appsv1.Deployment, error)
	deleteFunc func(name string, options *metav1.DeleteOptions) error
}

func (s *spyTelegrafDeploymentCUDer) Create(d *appsv1.Deployment) (*appsv1.Deployment, error) {
	return s.createFunc(d)
}

func (s *spyTelegrafDeploymentCUDer) Update(d *appsv1.Deployment) (*appsv1.Deployment, error) {
	return s.updateFunc(d)
}

func (s *spyTelegrafDeploymentCUDer) Delete(name string, options *metav1.DeleteOptions) error {
	return s.deleteFunc(name, options)
}

type spyRBACV1Client struct {
	spyRoleBindingCUDer
	spyRoleCUDer
}

func (s *spyRBACV1Client) RoleBindings(namespace string) typedrbacv1.RoleBindingInterface {
	return &s.spyRoleBindingCUDer
}

func (s *spyRBACV1Client) Roles(namespace string) typedrbacv1.RoleInterface {
	return &s.spyRoleCUDer
}

type spyRoleBindingCUDer struct {
	createFunc func(*rbacv1.RoleBinding) (*rbacv1.RoleBinding, error)
	deleteFunc func(name string, options *metav1.DeleteOptions) error
}

func (s *spyRoleBindingCUDer) Create(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	return s.createFunc(rb)
}

func (s *spyRoleBindingCUDer) Delete(name string, options *metav1.DeleteOptions) error {
	return s.deleteFunc(name, options)
}

type spyRoleCUDer struct {
	createFunc func(*rbacv1.Role) (*rbacv1.Role, error)
	deleteFunc func(name string, options *metav1.DeleteOptions) error
}

func (s *spyRoleCUDer) Create(r *rbacv1.Role) (*rbacv1.Role, error) {
	return s.createFunc(r)
}

func (s *spyRoleCUDer) Delete(name string, options *metav1.DeleteOptions) error {
	return s.deleteFunc(name, options)
}

func (spyRoleCUDer) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("this function should not be called")
}

func (spyRoleCUDer) Update(*rbacv1.Role) (*rbacv1.Role, error) {
	panic("this function should not be called")
}

func (spyRoleCUDer) Get(name string, options metav1.GetOptions) (*rbacv1.Role, error) {
	panic("this function should not be called")
}

func (spyRoleCUDer) List(opts metav1.ListOptions) (*rbacv1.RoleList, error) {
	panic("this function should not be called")
}

func (spyRoleCUDer) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("this function should not be called")
}

func (spyRoleCUDer) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *rbacv1.Role, err error) {
	panic("this function should not be called")
}

func (spyRoleBindingCUDer) Update(*rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	panic("this function should not be called")
}

func (spyRoleBindingCUDer) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("this function should not be called")
}

func (spyRoleBindingCUDer) Get(name string, options metav1.GetOptions) (*rbacv1.RoleBinding, error) {
	panic("this function should not be called")
}

func (spyRoleBindingCUDer) List(opts metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
	panic("this function should not be called")
}

func (spyRoleBindingCUDer) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("this function should not be called")
}

func (spyRoleBindingCUDer) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *rbacv1.RoleBinding, err error) {
	panic("this function should not be called")
}

func (s *spyConfigMapCUDer) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("this function should not be called")
}

func (s *spyConfigMapCUDer) Get(name string, options metav1.GetOptions) (*v1.ConfigMap, error) {
	panic("this function should not be called")
}

func (s *spyConfigMapCUDer) List(opts metav1.ListOptions) (*v1.ConfigMapList, error) {
	panic("this function should not be called")
}

func (s *spyConfigMapCUDer) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("this function should not be called")
}

func (s *spyConfigMapCUDer) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ConfigMap, err error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) UpdateStatus(*appsv1.Deployment) (*appsv1.Deployment, error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) Get(name string, options metav1.GetOptions) (*appsv1.Deployment, error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) List(opts metav1.ListOptions) (*appsv1.DeploymentList, error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *appsv1.Deployment, err error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) GetScale(deploymentName string, options metav1.GetOptions) (*autoscalingv1.Scale, error) {
	panic("this function should not be called")
}

func (s *spyTelegrafDeploymentCUDer) UpdateScale(deploymentName string, scale *autoscalingv1.Scale) (*autoscalingv1.Scale, error) {
	panic("this function should not be called")
}

func (s *spyPodDeleter) Create(*v1.Pod) (*v1.Pod, error) {
	panic("should not be called")
}

func (s *spyPodDeleter) Update(*v1.Pod) (*v1.Pod, error) {
	panic("should not be called")
}

func (s *spyPodDeleter) UpdateStatus(*v1.Pod) (*v1.Pod, error) {
	panic("should not be called")
}

func (s *spyPodDeleter) Delete(name string, options *metav1.DeleteOptions) error {
	panic("should not be called")
}

func (s *spyPodDeleter) Get(name string, options metav1.GetOptions) (*v1.Pod, error) {
	panic("should not be called")
}

func (s *spyPodDeleter) List(opts metav1.ListOptions) (*v1.PodList, error) {
	panic("should not be called")
}

func (s *spyPodDeleter) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("should not be called")
}

func (s *spyPodDeleter) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Pod, err error) {
	panic("should not be called")
}

func (s *spyPodDeleter) Bind(binding *v1.Binding) error {
	panic("should not be called")
}

func (s *spyPodDeleter) Evict(eviction *policyv1beta1.Eviction) error {
	panic("should not be called")
}

func (s *spyPodDeleter) GetLogs(name string, opts *v1.PodLogOptions) *rest.Request {
	panic("should not be called")
}
