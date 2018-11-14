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
package event

import (
	"expvar"
	"log"

	"k8s.io/api/core/v1"
)

var (
	ForwarderSent          *expvar.Int
	ForwarderFailed        *expvar.Int
	ForwarderConvertFailed *expvar.Int
)

func init() {
	ForwarderSent = expvar.NewInt("eventcontroller_forwarder_sent_count")
	ForwarderFailed = expvar.NewInt("eventcontroller_forwarder_failed_count")
	ForwarderConvertFailed = expvar.NewInt("eventcontroller_convert_failed_count")
}

type Forwarder interface {
	Post(string, interface{}) error
}

type Controller struct {
	f Forwarder
}

func NewController(l Forwarder) *Controller {
	return &Controller{
		f: l,
	}
}

func (c *Controller) AddFunc(o interface{}) {
	e, ok := o.(*v1.Event)
	if !ok {
		ForwarderConvertFailed.Add(1)
		log.Printf("got something other an event: %T\n", o)
		return
	}

	m := map[string]interface{}{
		"log":    []byte(e.Message),
		"stream": []byte("stdout"),
		"kubernetes": map[string]interface{}{
			"host":           []byte(e.Source.Host),
			"pod_name":       []byte(e.InvolvedObject.Name),
			"namespace_name": []byte(e.InvolvedObject.Namespace),
		},
	}

	err := c.f.Post("k8s.event", m)
	if err != nil {
		ForwarderFailed.Add(1)
		if ForwarderFailed.Value()%100 == 0 {
			log.Printf("unable to forward event: %s\n", err)
		}
		return
	}
	ForwarderSent.Add(1)
}

func (c *Controller) DeleteFunc(o interface{}) {
	// Do nothing!
}

func (c *Controller) UpdateFunc(o interface{}, n interface{}) {
	// Do nothing!
}
