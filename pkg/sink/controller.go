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
	"encoding/json"
	"log"
	"reflect"

	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Controller struct {
	cmp ConfigMapPatcher
	dsp DaemonSetPodDeleter
	sc  *Config
}

func NewController(cmp ConfigMapPatcher, dsp DaemonSetPodDeleter, sc *Config) *Controller {
	return &Controller{
		cmp: cmp,
		dsp: dsp,
		sc:  sc,
	}
}

func (c *Controller) OnAdd(o interface{}) {
	d, ok := o.(*v1alpha1.LogSink)
	if !ok {
		return
	}

	c.sc.UpsertSink(d)

	patches := []patch{
		{
			Op:    "replace",
			Path:  "/data/outputs.conf",
			Value: c.sc.String(),
		},
	}
	patchConfig(patches, c.cmp, c.dsp)
}

func (c *Controller) OnDelete(o interface{}) {
	d, ok := o.(*v1alpha1.LogSink)
	if !ok {
		return
	}

	c.sc.DeleteSink(d)

	patches := []patch{
		{
			Op:    "replace",
			Path:  "/data/outputs.conf",
			Value: c.sc.String(),
		},
	}
	patchConfig(patches, c.cmp, c.dsp)
}

func patchConfig(patches []patch, cmp ConfigMapPatcher, dsp DaemonSetPodDeleter) {
	data, err := json.Marshal(patches)
	if err != nil {
		log.Println(err.Error())
	}

	_, err = cmp.Patch(ConfigMapName, types.JSONPatchType, data)
	if err != nil {
		log.Println(err.Error())
	}

	err = dsp.DeleteCollection(
		nil,
		metav1.ListOptions{
			LabelSelector: "app=fluent-bit",
		},
	)
	if err != nil {
		log.Println(err.Error())
	}

}

func (c *Controller) OnUpdate(old, new interface{}) {
	o, ok := old.(*v1alpha1.LogSink)
	if !ok {
		return
	}
	n, ok := new.(*v1alpha1.LogSink)
	if !ok {
		return
	}
	if !reflect.DeepEqual(o.Spec, n.Spec) {
		c.OnAdd(new)
	}
}
