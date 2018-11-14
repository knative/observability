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
	_ "expvar"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/knative/observability/pkg/event"
)

type config struct {
	Host        string `env:"FORWARDER_HOST,required,report"`
	MetricsPort string `env:"METRICS_PORT,report"`
}

func main() {
	ctx := setupSignalHandler()

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
	eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.AddFunc,
		DeleteFunc: controller.DeleteFunc,
		UpdateFunc: controller.UpdateFunc,
	})

	eventInformer.Run(ctx.Done())
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
