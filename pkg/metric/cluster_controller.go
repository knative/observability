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
	"encoding/json"
	"log"
	"reflect"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// TODO: allow these to be configurable
	ConfigMapName  = "telegraf"
	DeploymentName = "telegraf"
)

type ConfigMapPatcher interface {
	Patch(
		name string,
		pt types.PatchType,
		data []byte,
		subresources ...string,
	) (*coreV1.ConfigMap, error)
}

type DaemonSetPodDeleter interface {
	DeleteCollection(
		options *metav1.DeleteOptions,
		listOptions metav1.ListOptions,
	) error
}

type patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type ClusterController struct {
	cmp ConfigMapPatcher
	dpd DaemonSetPodDeleter
	sc  *ClusterConfig
}

func NewClusterController(cmp ConfigMapPatcher, dpd DaemonSetPodDeleter, sc *ClusterConfig) *ClusterController {
	return &ClusterController{
		cmp: cmp,
		dpd: dpd,
		sc:  sc,
	}
}

func (c *ClusterController) OnAdd(o interface{}) {
	cmc, ok := o.(*v1alpha1.ClusterMetricSink)
	if !ok {
		return
	}

	c.sc.UpsertSink(*cmc)

	patches := []patch{
		{
			Op:    "replace",
			Path:  "/data/cluster-metric-sinks.conf",
			Value: c.sc.String(),
		},
	}

	data, err := json.Marshal(patches)
	if err != nil {
		log.Println(err.Error())
	}

	_, err = c.cmp.Patch(ConfigMapName, types.JSONPatchType, []byte(data))
	if err != nil {
		log.Println(err.Error())
	}

	err = c.dpd.DeleteCollection(
		nil,
		metav1.ListOptions{
			LabelSelector: "app=telegraf",
		},
	)
	if err != nil {
		log.Println(err.Error())
	}
}

func (c *ClusterController) OnDelete(o interface{}) {
	cmc, ok := o.(*v1alpha1.ClusterMetricSink)
	if !ok {
		return
	}

	c.sc.DeleteSink(*cmc)

	patches := []patch{
		{
			Op:    "replace",
			Path:  "/data/cluster-metric-sinks.conf",
			Value: c.sc.String(),
		},
	}

	data, err := json.Marshal(patches)
	if err != nil {
		log.Println(err.Error())
	}

	_, err = c.cmp.Patch(ConfigMapName, types.JSONPatchType, []byte(data))
	if err != nil {
		log.Println(err.Error())
	}

	err = c.dpd.DeleteCollection(
		nil,
		metav1.ListOptions{
			LabelSelector: "app=telegraf",
		},
	)
	if err != nil {
		log.Println(err.Error())
	}
}

func (c *ClusterController) OnUpdate(old, new interface{}) {
	if !reflect.DeepEqual(old, new) {
		c.OnAdd(new)
	}
}
