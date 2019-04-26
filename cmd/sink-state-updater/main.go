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
package main

import (
	"log"
	"time"

	"github.com/knative/observability/pkg/client/clientset/versioned"
	"github.com/knative/observability/pkg/state"
	"k8s.io/client-go/rest"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

type config struct {
	URL      string        `env:"SINK_STATE_URL,required,report"`
	Interval time.Duration `env:"POLL_INTERVAL,report"`
}

func main() {
	conf := config{
		Interval: 30 * time.Second,
	}
	err := envstruct.Load(&conf)
	if err != nil {
		log.Fatal(err.Error())
	}
	err = envstruct.WriteReport(&conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	stateClient := state.NewClient(conf.URL)

	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	observabilityClient, err := versioned.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	updater := state.NewUpdater(observabilityClient.ObservabilityV1alpha1())
	ticker := time.NewTicker(conf.Interval)
	defer ticker.Stop()
	for range ticker.C {
		states, err := stateClient.States()
		if err != nil {
			log.Printf("Failed to get sink states: %s", err)
			continue
		}

		updater.Update(state.FilterStaleStates(2*conf.Interval, states))
	}
}
