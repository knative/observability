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
	"flag"
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/knative/observability/pkg/client/clientset/versioned"
	informers "github.com/knative/observability/pkg/client/informers/externalversions"
	"github.com/knative/observability/pkg/sink"
	"github.com/knative/pkg/signals"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type config struct {
	Namespace string `env:"NAMESPACE,required,report"`
}

func main() {
	flag.Parse()
	stopCh := signals.SetupSignalHandler()

	var conf config
	err := envstruct.Load(&conf)
	if err != nil {
		log.Fatal(err.Error())
	}
	err = envstruct.WriteReport(&conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	client, err := versioned.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	coreV1Client, err := coreV1.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	sinkConfig := sink.NewConfig()

	controller := sink.NewController(
		coreV1Client.ConfigMaps(conf.Namespace),
		coreV1Client.Pods(conf.Namespace),
		sinkConfig,
	)

	clusterController := sink.NewClusterController(
		coreV1Client.ConfigMaps(conf.Namespace),
		coreV1Client.Pods(conf.Namespace),
		sinkConfig,
	)

	sinkInformerFactory := informers.NewSharedInformerFactory(client, time.Second*30)

	sinkInformer := sinkInformerFactory.Observability().V1alpha1().LogSinks().Informer()
	sinkInformer.AddEventHandler(controller)

	clusterSinkInformer := sinkInformerFactory.Observability().V1alpha1().ClusterLogSinks().Informer()
	clusterSinkInformer.AddEventHandler(clusterController)

	go sinkInformer.Run(stopCh)
	clusterSinkInformer.Run(stopCh)
}
