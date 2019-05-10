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
	"github.com/knative/observability/pkg/metric"
	"github.com/knative/pkg/signals"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/rest"
)

type config struct {
	Namespace                 string `env:"NAMESPACE,required,report"`
	UseInsecureKubernetesPort bool   `env:"USE_INSECURE_KUBERNETES_PORT,report"`
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

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	nodes, err := coreV1Client.Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}
	if len(nodes.Items) <= 0 {
		log.Fatal("cannot find any nodes")
	}
	clusterName := nodes.Items[0].Labels["pks-system/cluster.name"]

	metricSinkConfig := metric.NewConfig(clusterName, metric.KubernetesDefault(conf.UseInsecureKubernetesPort))

	cmsController := metric.NewClusterController(
		coreV1Client.ConfigMaps(conf.Namespace),
		coreV1Client.Pods(conf.Namespace),
		metricSinkConfig,
	)

	msController := metric.NewController(
		clusterName,
		coreV1Client,
		k8sClient.AppsV1(),
		k8sClient.RbacV1(),
	)

	sinkInformerFactory := informers.NewSharedInformerFactory(client, time.Second*30)

	cmsInformer := sinkInformerFactory.Observability().V1alpha1().ClusterMetricSinks().Informer()
	cmsInformer.AddEventHandler(cmsController)

	msInformer := sinkInformerFactory.Observability().V1alpha1().MetricSinks().Informer()
	msInformer.AddEventHandler(msController)

	go msInformer.Run(stopCh)
	cmsInformer.Run(stopCh)
}
