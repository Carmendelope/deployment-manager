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

// Watcher handles events from informers

package events

import (
	"fmt"

	"github.com/nalej/derrors"

	"github.com/rs/zerolog/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Watcher sets up and manages the lifecycle of an informer; it deals with
// events and makes sure the appropriate event handler is called.
// See NOTE in kubernetes.go
type Watcher struct {
	// The GroupVersionKind we're creating a watcher for
	gvk *schema.GroupVersionKind
	// The informer, containing store and controller
	informer cache.SharedIndexInformer
}

func NewWatcher(client rest.Interface, gvk *schema.GroupVersionKind, resource string, labelSelector string) (*Watcher, derrors.Error) {
	log.Debug().Str("kind", gvk.String()).Msg("new watcher")

	// Create empty object
	objType, err := scheme.Scheme.New(*gvk)
	if err != nil {
		return nil, derrors.NewInternalError(fmt.Sprintf("failed creating object for %s", gvk.String()), err)
	}

	// Don't filter for Events - they never have labels. If we want to
	// listen to these, we probably want to listen to all of them and
	// match them manually in the translator
	if gvk.Kind == "Event" {
		labelSelector = ""
	}

	// Check selectors
	parsedLabelSelector, err := labels.Parse(labelSelector)
	if err != nil {
		return nil, derrors.NewInternalError("failed parsing label selector", err)
	}

	// Create a lister-watcher
	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.Everything().String()
		options.LabelSelector = parsedLabelSelector.String()
	}

	watchlist := cache.NewFilteredListWatchFromClient(client, resource, metav1.NamespaceAll, optionsModifier)

	// Create an informer
	informer := cache.NewSharedIndexInformer(watchlist, objType, 0 /* No resync */, cache.Indexers{})

	watcher := &Watcher{
		gvk:      gvk,
		informer: informer,
	}

	return watcher, nil
}

func (w *Watcher) AddHandler(handler cache.ResourceEventHandler) {
	w.informer.AddEventHandler(handler)
}

func (w *Watcher) Start(stopChan <-chan struct{}) error {
	log.Debug().Str("resource", w.gvk.String()).Msg("starting watcher")
	go w.informer.Run(stopChan)

	// Wait for the caches to be synced
	if !cache.WaitForCacheSync(stopChan, w.informer.HasSynced) {
		return fmt.Errorf("Timed out waiting for Kubernetes event caches to sync")
	}

	log.Debug().Str("resource", w.gvk.String()).Msg("watcher synced and ready")

	return nil
}

func (w *Watcher) GetStore() cache.Store {
	return w.informer.GetStore()
}

func (w *Watcher) GetIndexer() cache.Indexer {
	return w.informer.GetIndexer()
}
