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
package sink

import (
	"reflect"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
)

type ClusterController struct {
	cmp ConfigMapPatcher
	dsp DaemonSetPodDeleter
	sc  *Config
}

func NewClusterController(cmp ConfigMapPatcher, dsp DaemonSetPodDeleter, sc *Config) *ClusterController {
	return &ClusterController{
		cmp: cmp,
		dsp: dsp,
		sc:  sc,
	}
}

func (c *ClusterController) AddFunc(o interface{}) {
	d, ok := o.(*v1alpha1.ClusterLogSink)
	if !ok {
		return
	}

	c.sc.UpsertClusterSink(d)

	patches := []patch{
		{
			Op:    "replace",
			Path:  "/data/outputs.conf",
			Value: c.sc.String(),
		},
	}
	patchConfig(patches, c.cmp, c.dsp)
}

func (c *ClusterController) DeleteFunc(o interface{}) {
	d, ok := o.(*v1alpha1.ClusterLogSink)
	if !ok {
		return
	}

	c.sc.DeleteClusterSink(d)

	patches := []patch{
		{
			Op:    "replace",
			Path:  "/data/outputs.conf",
			Value: c.sc.String(),
		},
	}
	patchConfig(patches, c.cmp, c.dsp)
}

func (c *ClusterController) UpdateFunc(old, new interface{}) {
	if !reflect.DeepEqual(old, new) {
		c.AddFunc(new)
	}
}
