/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

// Dispatches Kubernetes events functions from a DispatchFuncs

package events

import (
	"fmt"
	"reflect"

	"github.com/nalej/derrors"

	"github.com/rs/zerolog/log"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

        "k8s.io/client-go/kubernetes/scheme"
        "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Maximum number of times to retry processing an event
const maxRetries = 5

type KindList []schema.GroupVersionKind

// Actions that a Kubernetes event can describe
type EventType string
const (
	EventAdd EventType = "add"
	EventUpdate EventType = "update"
	EventDelete EventType = "delete"
)

func (e EventType) String() string {
	return string(e)
}

type Event struct {
	Key string
	OldObjKey string
	Kind schema.GroupVersionKind
	EventType EventType
}

// Interface for a collection of dispatcher functions. As an implementation
// can contain an arbirtary number of functions, the only interface-defined
// method is to list the GroupVersionKinds this function collection supports.
type DispatchFuncs interface {
	SupportedKinds() KindList
	// Link the client resource store to the translator for cross-referencing objects
	SetStore(kind schema.GroupVersionKind, store cache.Store) error
}

// Function type for the SupportedKinds of a DispatchFuncs collection
type DispatchFunc func(oldObj, newObj interface{}, action EventType) error

// Dispatcher implements k8s.io/client-go/tools/cache.ResourceEventHandler
// so it can be used directly in the informer.
type Dispatcher struct {
	// Interface to the dispatcher functions struct
	dispatchFuncs DispatchFuncs

	// The mapping from kind to function pointer
	funcMap map[schema.GroupVersionKind]DispatchFunc

	// Event handling queue
	queue workqueue.RateLimitingInterface

	// Communicate events from queue to dispatcher
	eventChan chan Event

	// Indexers to retrieve objects from - created by informers
	indexers map[schema.GroupVersionKind]cache.Indexer

	// Index for storing deleted objects
	deletedIndexer cache.Indexer
}

func NewDispatcher(funcs DispatchFuncs) (*Dispatcher, derrors.Error) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	kinds := funcs.SupportedKinds()

	dispatcher := &Dispatcher{
		dispatchFuncs: funcs,
		funcMap: make(map[schema.GroupVersionKind]DispatchFunc, len(kinds)),
		queue: queue,
		indexers: make(map[schema.GroupVersionKind]cache.Indexer, len(kinds)),
		deletedIndexer: cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{}),
	}

	funcsValue := reflect.ValueOf(funcs)

	// Create the dynamic function map to this exact instance
	for _, kind := range(kinds) {
		fName := fmt.Sprintf("On%s", kind.Kind)
		fValue := funcsValue.MethodByName(fName)
		if !fValue.IsValid() {
			return nil, derrors.NewInternalError(fmt.Sprintf("function %s not defined in dispatchfuncs", fName))
		}
		dispatcher.funcMap[kind] = fValue.Interface().(func(interface{}, interface{}, EventType) error)
	}

	return dispatcher, nil
}

func (d *Dispatcher) Dispatchable() KindList {
	kinds := make(KindList, 0, len(d.funcMap))
	for k, _ := range(d.funcMap) {
		kinds = append(kinds, k)
	}

	return kinds
}

func (d *Dispatcher) Start(stopChan <-chan struct{}) error {
	log.Debug().Msg("starting dispatcher")
	go d.Run(stopChan)
	return nil
}

func (d *Dispatcher) Run(stopChan <-chan struct{}) {
	// Let the workers stop when we are done
	defer d.queue.ShutDown()

	go d.worker()

	<-stopChan
	log.Debug().Msg("stopping dispatcher")
}

func (d *Dispatcher) SetStore(kind schema.GroupVersionKind, store cache.Store) error {
	return d.dispatchFuncs.SetStore(kind, store)
}

func (d *Dispatcher) SetIndexer(kind schema.GroupVersionKind, indexer cache.Indexer) error {
	_, found := d.indexers[kind]
	if found {
		return fmt.Errorf("Store for %s already set", kind.Kind)
	}

	d.indexers[kind] = indexer

	return nil
}

func (d *Dispatcher) OnAdd(obj interface{}) {
	e, err := createEvent(obj, EventAdd)
	if err != nil {
		return
	}

	d.queue.Add(e)
}

func (d *Dispatcher) OnUpdate(oldObj, newObj interface{}) {
	e, err := createEvent(newObj, EventUpdate)
	if err != nil {
		return
	}

	/* Store reference to original object as well */
	oldObjKey, err := cache.MetaNamespaceKeyFunc(oldObj)
	if err != nil {
		log.Error().Err(err).Interface("oldObj", oldObj).Msg("unable to create event key")
		return
	}
	e.OldObjKey = oldObjKey

	d.queue.Add(e)
}

func (d *Dispatcher) OnDelete(obj interface{}) {
	e, err := createEvent(obj, EventDelete)
	if err != nil {
		return
	}

	// We store this object ourselves because it won't be available in the
	// regular cache
	d.deletedIndexer.Add(obj)

	d.queue.Add(e)
}

func (d *Dispatcher) worker() {
	for {
		event, quit := d.queue.Get()
		if quit {
			// Queue has been stopped by Run() after stopChan has
			// been closed.
			break
		}

		err := d.dispatch(event.(Event))
		if err == nil {
			d.queue.Forget(event)
		} else if d.queue.NumRequeues(event) < maxRetries {
			log.Warn().Err(err).Interface("event", event).Msg("Error processing event. Retrying.")
			d.queue.AddRateLimited(event)
		} else {
			log.Error().Err(err).Interface("event", event).Msg("Error processing event. Giving up.")
			d.queue.Forget(event)
		}

		// Tell the queue that we are done with processing this key. This unblocks the key for other workers
		// This allows safe parallel processing because two pods with the same key are never processed in
		// parallel.
		d.queue.Done(event)
	}
}

func (d *Dispatcher) dispatch(event Event) error {
	oldObj, obj, derr := d.fetchObj(&event)
	if derr != nil {
		return derr
	}

	f, found := d.funcMap[event.Kind]
	if !found {
		return derrors.NewInvalidArgumentError("no translator found").WithParams(event.Kind)
	}

	// Get some metadata for useful logging
	meta, err := meta.Accessor(obj)
	if err != nil {
		log.Debug().Interface("obj", obj).Msg("error getting object metadata")
		return err
	}

	log.Debug().Str("resource", meta.GetSelfLink()).Str("event", event.EventType.String()).Msg("dispatching")
	return f(oldObj, obj, event.EventType)
}

func (d *Dispatcher) fetchObj(event *Event) (interface{}, interface{}, derrors.Error) {
	indexer, found := d.indexers[event.Kind]
	if !found {
		return nil, nil, derrors.NewInvalidArgumentError("no indexer found").WithParams(event.Kind)
	}

	obj, exists, err := indexer.GetByKey(event.Key)
	if err != nil {
		return nil, nil, derrors.NewInternalError("error fetching key from store", err).WithParams(event.Key)
	}

	// Check if deleted
	if !exists {
		obj, exists, err = d.deletedIndexer.GetByKey(event.Key)
		if err != nil {
			return nil, nil, derrors.NewInternalError("error fetching key from deleted object store", err).WithParams(event.Key)
		}
		if !exists {
			return nil, nil, derrors.NewInvalidArgumentError("key does not exist in deleted object store while it should")
		}
		d.deletedIndexer.Delete(event.Key)
	}

	var oldObj interface{} = nil
	if event.OldObjKey != "" {
		oldObj, _, err = indexer.GetByKey(event.Key)
		if err != nil {
			return nil, nil, derrors.NewInternalError("error fetching key from store", err).WithParams(event.Key)
		}
		// Ok if old object does not exist and is nil
	}

	return oldObj, obj, nil
}

func getKind(obj interface{}) (schema.GroupVersionKind, error) {
	// Get some metadata for useful logging
	meta, err := meta.Accessor(obj)
	if err != nil {
		log.Error().Err(err).Msg("non-kubernetes object received")
		return schema.GroupVersionKind{}, err
	}

	kinds, _, err := scheme.Scheme.ObjectKinds(obj.(runtime.Object))
	if err != nil {
		log.Warn().Str("resource", meta.GetSelfLink()).Msg("invalid object received")
		return schema.GroupVersionKind{}, err
	}

	// Not sure what to do if an object matches multiple kinds, let's
	// at least warn
	if len(kinds) > 1 {
		kindLog := log.Warn().Str("resource", meta.GetSelfLink())
		for _, k := range(kinds) {
			kindLog = kindLog.Str("candidate", k.String())
		}
		kindLog.Msg("received ambiguous object, picking first candidate")
	}

	kind := kinds[0]

	return kind, nil
}

func createEvent(obj interface{}, eventType EventType) (Event, error) {
	// This is the key func the default Indexer uses for all events
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Error().Err(err).Interface("obj", obj).Msg("unable to create event key")
		return Event{}, err
	}

	kind, err := getKind(obj)
	if err != nil {
		log.Error().Err(err).Interface("obj", obj).Msg("unable to determine object kind")
		return Event{}, err
	}

	e := Event{
		Key: key,
		Kind: kind,
		EventType: eventType,
	}

	return e, nil
}

