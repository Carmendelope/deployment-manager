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

// Translates event actions to platform metrics

package kubernetes

import (
	"fmt"

	"github.com/nalej/deployment-manager/pkg/kubernetes/events"
	"github.com/nalej/deployment-manager/pkg/metrics"
	"github.com/nalej/deployment-manager/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/rs/zerolog/log"
)

// We'll filter out these labels
var FilterLabelSet = map[string]string{
	"agent": "zt-agent",
}

type MetricsTranslator struct {
	// Collect metrics based on events
	collector metrics.Collector
	// Server startup
	startupTime metav1.Time

	// We have a reference back to the Data stores of the informers for
	// each kind of resource we support. We use this to look up and
	// cross-reference resources to figure out the needed translation.
	stores map[string]cache.Store
}

// NOTE: On start, we count the incoming create/delete to update
// total running, but created/deleted should be 0 to properly use
// the prometheus counters
func NewMetricsTranslator(collector metrics.Collector) *MetricsTranslator {
	return &MetricsTranslator{
		collector:   collector,
		startupTime: metav1.Now(),
		stores:      map[string]cache.Store{},
	}
}

func (t *MetricsTranslator) SetStore(kind schema.GroupVersionKind, store cache.Store) error {
	_, found := t.stores[kind.Kind]
	if found {
		return fmt.Errorf("Store for %s already set", kind.Kind)
	}

	t.stores[kind.Kind] = store
	return nil
}

func (t *MetricsTranslator) SupportedKinds() events.KindList {
	return events.KindList{
		DeploymentKind,
		NamespaceKind,
		PVCKind,
		// We only watch this so we have the resource store
		PodKind,
		ServiceKind,
		EventKind,
		IngressKind,
	}
}

// Translating functions
func (t *MetricsTranslator) OnDeployment(oldObj, obj interface{}, action events.EventType) error {
	d := obj.(*appsv1.Deployment)

	// filter out zt-agent deployment
	if !isAppInstance(d) {
		return nil
	}

	return t.translate(action, metrics.MetricServices, &d.CreationTimestamp)
}

func (t *MetricsTranslator) OnNamespace(oldObj, obj interface{}, action events.EventType) error {
	n := obj.(*corev1.Namespace)

	return t.translate(action, metrics.MetricFragments, &n.CreationTimestamp)
}

func (t *MetricsTranslator) OnPersistentVolumeClaim(oldObj, obj interface{}, action events.EventType) error {
	pvc := obj.(*corev1.PersistentVolumeClaim)
	return t.translate(action, metrics.MetricVolumes, &pvc.CreationTimestamp)
}

func (t *MetricsTranslator) OnPod(oldObj, obj interface{}, action events.EventType) error {
	// No action - only watched to have the resource store for reference
	return nil
}

func (t *MetricsTranslator) OnIngress(oldObj, obj interface{}, action events.EventType) error {
	i := obj.(*extensionsv1beta1.Ingress)
	return t.translate(action, metrics.MetricEndpoints, &i.CreationTimestamp)
}

func (t *MetricsTranslator) OnService(oldObj, obj interface{}, action events.EventType) error {
	s := obj.(*corev1.Service)

	if s.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}

	return t.translate(action, metrics.MetricEndpoints, &s.CreationTimestamp)
}

// NOTE: At this point we log Warning events as errors. For true errors we would
// need to decide what an error actually is (unavailable container or endpoint?
// application that quits unexpectedly?), if it's transient or permanent,
// whether we actually care about it, etc. Then we'd need to analyze the event
// and other resources to figure out what we're dealing with. So, for now, we
// just count warnings.
func (t *MetricsTranslator) OnEvent(oldObj, obj interface{}, action events.EventType) error {
	e := obj.(*corev1.Event)

	if action == events.EventDelete {
		return nil
	}

	// Discard any normal events, and any events that
	// happened before we started watching (to avoid double counting after
	// a restart)
	if e.Type != "Warning" || e.LastTimestamp.Before(&t.startupTime) {
		return nil
	}

	if action == events.EventUpdate {
		oldE := oldObj.(*corev1.Event)
		// If count increased, we log another warning
		if oldE.Count == e.Count {
			return nil
		}
	}

	// Get object event references
	ref, exists := t.getReferencedObject(&e.InvolvedObject)
	if !exists {
		return nil
	}

	// Check if the referred object is of interest
	if !isAppInstance(ref) {
		return nil
	}

	kind := e.InvolvedObject.Kind
	log.Debug().Str("object", kind).Str("name", e.InvolvedObject.Name).
		Int32("count", e.Count).Str("reason", e.Reason).Str("message", e.Message).Msg("event")

	switch kind {
	case PodKind.Kind:
		// Filter out references to zt container
		if e.InvolvedObject.FieldPath == "spec.containers{zt-sidecar}" {
			return nil
		}
		fallthrough
	case DeploymentKind.Kind:
		t.collector.Error(metrics.MetricServices)

	case ServiceKind.Kind:
		s := ref.(*corev1.Service)
		if s.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return nil
		}
		fallthrough
	case IngressKind.Kind:
		t.collector.Error(metrics.MetricEndpoints)

	case PVCKind.Kind:
		t.collector.Error(metrics.MetricVolumes)

	case NamespaceKind.Kind:
		t.collector.Error(metrics.MetricFragments)
	}

	return nil
}

func (t *MetricsTranslator) getReferencedObject(ref *corev1.ObjectReference) (interface{}, bool) {
	// If we don't have a store for this kind, we are not intereseted in
	// the object - we also cannot easily retrieve it.
	store, found := t.stores[ref.Kind]
	if !found {
		return nil, false
	}

	var key string
	if len(ref.Namespace) > 0 {
		key = fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
	} else {
		key = ref.Name
	}

	obj, exists, err := store.GetByKey(key)
	if err != nil {
		log.Error().Err(err).Msg("error retrieving object from resource store")
		return nil, false
	}
	if !exists {
		return nil, false
	}

	return obj, true
}

// Calls create function if after server startup, existing otherwise
func (t *MetricsTranslator) translate(action events.EventType, metric metrics.MetricType, ts *metav1.Time) error {
	switch action {
	case events.EventAdd:
		if t.startupTime.Before(ts) {
			t.collector.Create(metric)
		} else {
			t.collector.Existing(metric)
		}
	case events.EventDelete:
		t.collector.Delete(metric)
	}

	return nil
}

// Filter out non-application-instance objects
func isAppInstance(obj interface{}) bool {
	metaobj, err := meta.Accessor(obj)
	if err != nil {
		log.Error().Err(err).Msg("invalid object retrieved from resource store")
		return false
	}
	labels := metaobj.GetLabels()
	_, found := labels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID]
	if !found {
		log.Debug().Msg("no nalej-organization")
		return false
	}

	// filter out unwanted instances
	for k, v := range FilterLabelSet {
		label, found := labels[k]
		if found && label == v {
			return false
		}
	}

	return true
}
