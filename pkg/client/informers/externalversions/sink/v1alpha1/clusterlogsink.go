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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	sinkv1alpha1 "github.com/knative/observability/pkg/apis/sink/v1alpha1"
	versioned "github.com/knative/observability/pkg/client/clientset/versioned"
	internalinterfaces "github.com/knative/observability/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/knative/observability/pkg/client/listers/sink/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterLogSinkInformer provides access to a shared informer and lister for
// ClusterLogSinks.
type ClusterLogSinkInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ClusterLogSinkLister
}

type clusterLogSinkInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewClusterLogSinkInformer constructs a new informer for ClusterLogSink type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterLogSinkInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterLogSinkInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredClusterLogSinkInformer constructs a new informer for ClusterLogSink type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterLogSinkInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ObservabilityV1alpha1().ClusterLogSinks(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ObservabilityV1alpha1().ClusterLogSinks(namespace).Watch(options)
			},
		},
		&sinkv1alpha1.ClusterLogSink{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterLogSinkInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterLogSinkInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterLogSinkInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&sinkv1alpha1.ClusterLogSink{}, f.defaultInformer)
}

func (f *clusterLogSinkInformer) Lister() v1alpha1.ClusterLogSinkLister {
	return v1alpha1.NewClusterLogSinkLister(f.Informer().GetIndexer())
}
