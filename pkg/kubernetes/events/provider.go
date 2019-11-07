/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Kubernetes Events provider

package events

import (
	"fmt"

	"github.com/nalej/derrors"

	"github.com/rs/zerolog/log"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// EventsProvider implements the EventsProvider interface; it
// subscribes to Kubernetes events and translates each incoming event to
// a platform metric
type EventsProvider struct {
	// Configuration to create Kubernetes client
	kubeconfig *rest.Config

	// Cached Kubernetes clients for event subscription
	clients map[schema.GroupVersion]rest.Interface

	// Cached Kubernetes informers for handler subscription
	watchers map[schema.GroupVersionKind]*Watcher

	// Registered dispatchers that need to be started and stopped
	dispatchers []*Dispatcher

	// Mapper for creating watcher for the right REST endpoints
	restMapper meta.RESTMapper

	// Filters for the watchers
	labelSelector string

	// Channel to stop informers. Close to stop.
	stopChan chan struct{}

	// watchers have started
	started bool
}

// NOTE: There is a simple example on how to deal with Kubernetes events here:
//   https://rsmitty.github.io/Kubernetes-Events/
// There is a more complex example here:
//   https://engineering.bitnami.com/articles/kubewatch-an-example-of-kubernetes-custom-controller.html
//
// At this moment I don't think we need a workqueue and a rate limiter - we
// need all the events and the work we do really isn't that much (we pretty
// much just determine event type and increase a counter). Furthermore, I don't
// think we need to bother with a shared informer, as we only have one handler
// per informer anyway; nor do we need an index as we don't have a queue and
// can pass the object straight to the handler.
// We can introduce these concepts when needed for optimization.
//
// An extensive description of the event mechanism can be found here:
//   https://lairdnelson.wordpress.com/2018/01/07/understanding-kubernetes-tools-cache-package-part-0/

func NewEventsProvider(configfile string, incluster bool, labelSelector string) (*EventsProvider, derrors.Error) {
	log.Debug().Str("config", configfile).Bool("in-cluster", incluster).Msg("creating kubernetes events provider")

	var kubeconfig *rest.Config
	var err error
	if incluster {
		kubeconfig, err = rest.InClusterConfig()
	} else {
		kubeconfig, err = clientcmd.BuildConfigFromFlags("", configfile)
	}
	if err != nil {
		return nil, derrors.NewInternalError("failed to create kubeclient configuration", err)
	}
	log.Info().Str("host", kubeconfig.Host).Msg("created kubeconfig")

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return nil, derrors.NewInternalError("failed to create discovery client", err)
	}
	resources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, derrors.NewInternalError("failed to get api group resources", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(resources)

	provider := &EventsProvider{
		kubeconfig:    kubeconfig,
		clients:       map[schema.GroupVersion]rest.Interface{},
		watchers:      map[schema.GroupVersionKind]*Watcher{},
		dispatchers:   []*Dispatcher{},
		restMapper:    mapper,
		labelSelector: labelSelector,
		stopChan:      make(chan struct{}),
	}
	return provider, nil
}

// Start collecting metrics
func (p *EventsProvider) Start() derrors.Error {
	log.Info().Msg("starting kubernetes events listener")

	p.started = true

	// Start all watchers - this syncs the Kubernetes event caches as well
	for _, watcher := range p.watchers {
		err := watcher.Start(p.stopChan)
		if err != nil {
			p.Stop()
			return derrors.NewInternalError("unable to start watcher", err)
		}
	}

	// Start all processing queue items in dispatchers
	for _, dispatcher := range p.dispatchers {
		err := dispatcher.Start(p.stopChan)
		if err != nil {
			p.Stop()
			return derrors.NewInternalError("unable to start dispatcher", err)
		}
	}

	return nil
}

func (p *EventsProvider) AddDispatcher(dispatcher *Dispatcher) derrors.Error {
	log.Debug().Msg("adding dispatcher to events listener")
	// Note - not safe under concurrency - we can add start _while_ adding
	// a dispatcher. If we need this to be thread safe, we need to lock.
	// Likely we just want to figure out if we can allow adding dispatchers
	// at any time - this is now more of a fail safe because I don't want
	// to think about how this would work.
	if p.started {
		return derrors.NewInvalidArgumentError("cannot add dispatchers after event provider has started")
	}

	// Set up watchers
	for _, kind := range dispatcher.Dispatchable() {
		// Get watcher
		watcher, derr := p.createWatcher(kind)
		if derr != nil {
			return derr
		}

		// Link the client resource store to the translator for
		// cross-referencing objects
		err := dispatcher.SetStore(kind, watcher.GetStore())
		if err != nil {
			return derrors.NewAlreadyExistsError("store already set", err)
		}

		// Set indexer for this specific kind to be able to retrieve
		// objects based on key
		err = dispatcher.SetIndexer(kind, watcher.GetIndexer())
		if err != nil {
			return derrors.NewAlreadyExistsError("indexer already set", err)
		}

		watcher.AddHandler(dispatcher)
	}

	p.dispatchers = append(p.dispatchers, dispatcher)

	return nil
}

func (p *EventsProvider) createWatcher(gvk schema.GroupVersionKind) (*Watcher, derrors.Error) {
	watcher, found := p.watchers[gvk]
	if found {
		return watcher, nil
	}

	// Create cached client
	client, derr := p.createClient(gvk.GroupVersion())
	if derr != nil {
		return nil, derr
	}

	// Figure out resource with RESTMapper
	mapping, err := p.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, derrors.NewInternalError("unable to get rest mapping", err)
	}
	resource := mapping.Resource.Resource

	watcher, derr = NewWatcher(client, &gvk, resource, p.labelSelector)
	if derr != nil {
		return nil, derr
	}

	p.watchers[gvk] = watcher

	return watcher, nil
}

func (p *EventsProvider) createClient(gv schema.GroupVersion) (rest.Interface, derrors.Error) {
	client, found := p.clients[gv]
	if found {
		return client, nil
	}

	log.Debug().Str("gv", gv.String()).Msg("creating new client")

	// Create shallow copy
	c := *p.kubeconfig

	c.GroupVersion = &gv

	// The core api has no group and has a slightly different base URL
	if gv.Group != "" {
		c.APIPath = "/apis"
	} else {
		c.APIPath = "/api"
	}
	c.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if c.UserAgent == "" {
		c.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	client, err := rest.RESTClientFor(&c)
	if err != nil {
		return nil, derrors.NewInternalError(fmt.Sprintf("failed creating kubernetes client for %s", gv.String()), err)
	}

	p.clients[gv] = client
	return client, nil
}

// Stop collecting metrics
func (p *EventsProvider) Stop() derrors.Error {
	log.Info().Msg("stopping kubernetes event provider")
	// Stop informers
	close(p.stopChan)
	return nil
}
