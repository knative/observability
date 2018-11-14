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
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/knative/observability/pkg/client/clientset/versioned"
	informers "github.com/knative/observability/pkg/client/informers/externalversions"
	"github.com/knative/observability/pkg/sink"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type config struct {
	Namespace string `env:"NAMESPACE,required,report"`
}

func main() {
	flag.Parse()
	ctx := setupSignalHandler()

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
	sinkInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.AddFunc,
		DeleteFunc: controller.DeleteFunc,
		UpdateFunc: controller.UpdateFunc,
	})

	clusterSinkInformer := sinkInformerFactory.Observability().V1alpha1().ClusterLogSinks().Informer()
	clusterSinkInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    clusterController.AddFunc,
		DeleteFunc: clusterController.DeleteFunc,
		UpdateFunc: clusterController.UpdateFunc,
	})

	go sinkInformer.Run(ctx.Done())
	clusterSinkInformer.Run(ctx.Done())
}

var onlyOneSignalHandler = make(chan struct{})

// setupSignalHandler registers SIGTERM and SIGINT. A context is returned
// which is canceled on one of these signals. If a second signal is caught,
// the program is terminated with exit code 1.
func setupSignalHandler() context.Context {
	close(onlyOneSignalHandler) // only call once, panic on calls > 1

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
