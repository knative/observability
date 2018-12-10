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
	_ "expvar"
	"log"
	"net"
	"net/http"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/knative/observability/pkg/event"
	"github.com/knative/pkg/signals"
)

type config struct {
	Host        string `env:"FORWARDER_HOST,required,report"`
	MetricsPort string `env:"METRICS_PORT,report"`
}

func main() {
	stopCh := signals.SetupSignalHandler()

	conf := config{
		MetricsPort: "6060",
	}
	err := envstruct.Load(&conf)
	if err != nil {
		log.Fatal(err.Error())
	}
	err = envstruct.WriteReport(&conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	go http.ListenAndServe(net.JoinHostPort("", conf.MetricsPort), http.DefaultServeMux)

	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	kclientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	f, err := fluent.New(fluent.Config{
		FluentHost: conf.Host,
	})
	if err != nil {
		log.Fatalf("unable to create fluent logger client: %s", err)
	}

	controller := event.NewController(f)

	informerFactory := informers.NewSharedInformerFactory(kclientset, 30*time.Second)

	eventInformer := informerFactory.Core().V1().Events().Informer()
	eventInformer.AddEventHandler(controller)

	eventInformer.Run(stopCh)
}
