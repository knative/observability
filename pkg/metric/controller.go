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
package metric

import (
	"fmt"
	"log"
	"reflect"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	typedrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

// This is a build arg that's injected with the appropriate SHA of
// the telegraf image
var TelegrafImageVersion string = "1.11-alpine"

type V1CoreClient interface {
	typedv1.ConfigMapsGetter
	typedv1.PodsGetter
}

type V1beta1ExtensionsClient interface {
	typedappsv1.DeploymentsGetter
}

type RBACV1Client interface {
	typedrbacv1.RolesGetter
	typedrbacv1.RoleBindingsGetter
}

type Controller struct {
	coreClient       V1CoreClient
	extensionsClient V1beta1ExtensionsClient
	rbacV1Client     RBACV1Client
	clusterName      string
}

func NewController(clusterName string, c V1CoreClient, d V1beta1ExtensionsClient, r RBACV1Client) *Controller {
	log.Printf("Using telegraf:%s for metric sink deployments", TelegrafImageVersion)
	return &Controller{
		clusterName:      clusterName,
		coreClient:       c,
		extensionsClient: d,
		rbacV1Client:     r,
	}
}

func (c *Controller) OnAdd(o interface{}) {
	ms, ok := o.(*v1alpha1.MetricSink)
	if !ok {
		return
	}

	setDefaultTypeMeta(ms)

	_, err := c.rbacV1Client.Roles(ms.Namespace).Create(getTelegrafRole(ms))
	if err != nil {
		log.Printf("Unable to create role: %s\n", err)
		return
	}

	_, err = c.rbacV1Client.RoleBindings(ms.Namespace).Create(getTelegrafRoleBinding(ms))
	if err != nil {
		log.Printf("Unable to create role binding: %s\n", err)
		return
	}

	_, err = c.coreClient.ConfigMaps(ms.Namespace).Create(c.getTelegrafConfigMap(ms))
	if err != nil {
		log.Printf("Unable to create config map: %s\n", err)
		return
	}

	_, err = c.extensionsClient.Deployments(ms.Namespace).Create(getTelegrafDeployment(ms))
	if err != nil {
		log.Printf("Unable to create deployment: %s\n", err)
		return
	}
}

func (c *Controller) OnUpdate(o, n interface{}) {
	oms, ok := o.(*v1alpha1.MetricSink)
	if !ok {
		return
	}
	nms, ok := n.(*v1alpha1.MetricSink)
	if !ok {
		return
	}
	if reflect.DeepEqual(oms.Spec, nms.Spec) {
		return
	}

	// TODO: Should we do a patch instead?
	_, err := c.coreClient.ConfigMaps(nms.Namespace).Update(c.getTelegrafConfigMap(nms))
	if err != nil {
		log.Printf("Unable to update config map: %s\n", err)
		return
	}

	err = c.coreClient.Pods(nms.Namespace).DeleteCollection(
		nil,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", getAppName(nms)),
		},
	)
	if err != nil {
		log.Printf("Unable to delete pod collection: %s\n", err)
		return
	}
}

func (c *Controller) OnDelete(o interface{}) {
	ms, ok := o.(*v1alpha1.MetricSink)
	if !ok {
		return
	}

	name := getAppName(ms)
	err := c.coreClient.ConfigMaps(ms.Namespace).Delete(name, nil)
	if err != nil {
		log.Printf("Unable to delete config map: %s\n", err)
		return
	}

	err = c.extensionsClient.Deployments(ms.Namespace).Delete(name, nil)
	if err != nil {
		log.Printf("Unable to delete deployment: %s\n", err)
		return
	}

	err = c.rbacV1Client.RoleBindings(ms.Namespace).Delete(name, nil)
	if err != nil {
		log.Printf("Unable to delete role binding: %s\n", err)
		return
	}

	err = c.rbacV1Client.Roles(ms.Namespace).Delete(name, nil)
	if err != nil {
		log.Printf("Unable to delete role: %s\n", err)
		return
	}
}

func (c *Controller) getTelegrafConfigMap(ms *v1alpha1.MetricSink) *v1.ConfigMap {
	name := getAppName(ms)
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ms.Namespace,
			Labels:    map[string]string{"app": name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: ms.APIVersion,
				Kind:       ms.Kind,
				Name:       ms.Name,
				UID:        ms.UID,
			}},
		},
		Data: map[string]string{
			"telegraf.conf":     DefaultTelegrafConf,
			"metric-sinks.conf": c.metricSinkConfig(ms),
		},
	}
}

func getAppName(ms *v1alpha1.MetricSink) string {
	return fmt.Sprintf("telegraf-%s", ms.Name)
}

func getTelegrafDeployment(ms *v1alpha1.MetricSink) *appsv1.Deployment {
	var r int32 = 1
	name := getAppName(ms)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			ClusterName: ms.ClusterName,
			Name:        name,
			Namespace:   ms.Namespace,
			Labels:      map[string]string{"app": name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: ms.APIVersion,
				Kind:       ms.Kind,
				Name:       ms.Name,
				UID:        ms.UID,
			}},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Replicas: &r,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{{
						Name: "telegraf-config",
						VolumeSource: v1.VolumeSource{
							ConfigMap: &v1.ConfigMapVolumeSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: name,
								},
							},
						},
					}},
					Containers: []v1.Container{{
						Name:    "telegraf",
						Image:   "telegraf:" + TelegrafImageVersion,
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
}

func getTelegrafRoleBinding(ms *v1alpha1.MetricSink) *rbacv1.RoleBinding {
	name := getAppName(ms)
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ms.Namespace,
			Labels:    map[string]string{"app": name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: ms.APIVersion,
				Kind:       ms.Kind,
				Name:       ms.Name,
				UID:        ms.UID,
			}},
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: ms.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     name,
		},
	}
}

func getTelegrafRole(ms *v1alpha1.MetricSink) *rbacv1.Role {
	name := getAppName(ms)
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ms.Namespace,
			Labels:    map[string]string{"app": name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: ms.APIVersion,
				Kind:       ms.Kind,
				Name:       ms.Name,
				UID:        ms.UID,
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
}

func (c *Controller) metricSinkConfig(ms *v1alpha1.MetricSink) string {
	config := telegrafConfig{
		Inputs:  make(map[string][]map[string]interface{}),
		Outputs: make(map[string][]map[string]interface{}),
	}

	if c.clusterName != "" {
		config.GlobalTags = map[string]string{"cluster_name": c.clusterName}
	}

	config.Inputs["prometheus"] = []map[string]interface{}{{"monitor_kubernetes_pods": true, "monitor_kubernetes_pods_namespace": ms.Namespace}}

	appendInputsAndOutputs(&config, ms.Spec.Inputs, ms.Spec.Outputs)

	return config.String()
}

func setDefaultTypeMeta(ms *v1alpha1.MetricSink) {
	if ms.Kind == "" {
		ms.Kind = "MetricSink"
	}
	if ms.APIVersion == "" {
		ms.APIVersion = "observability.knative.dev/v1alpha1"
	}
}

const DefaultTelegrafConf = `
[agent]
  interval = "10s"
  round_interval = true
  metric_batch_size = 1000
  metric_buffer_limit = 10000
  collection_jitter = "0s"
  flush_interval = "10s"
  flush_jitter = "0s"
  precision = ""
  debug = false
  quiet = false
  logfile = ""
  hostname = ""
  omit_hostname = false`
