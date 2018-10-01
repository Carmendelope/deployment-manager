/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    "fmt"
    "time"

    "github.com/golang/glog"

    //"k8s.io/api/core/v1"
    //meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/apimachinery/pkg/fields"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"

    "k8s.io/api/extensions/v1beta1"
    "github.com/rs/zerolog/log"
    "github.com/nalej/deployment-manager/pkg/executor"
    "k8s.io/api/core/v1"
)

// The kubernetes controllers has a set of queues monitoring k8s related operations.
type KubernetesController struct {
    // Deployments controller
    deployments *KubernetesObserver
    // Services controller
    services *KubernetesObserver
    // Pending checks to run
    pendingStages *executor.PendingStages
}


// Create a new kubernetes controller for a given namespace.
func NewKubernetesController(executor *KubernetesExecutor, pendingStages *executor.PendingStages, namespace string) executor.DeploymentController {

    // Watch deployments
    deploymentsListWatcher := cache.NewListWatchFromClient(
        executor.client.ExtensionsV1beta1().RESTClient(),
        "deployments", namespace, fields.Everything())
    depObserver := NewKubernetesObserver(deploymentsListWatcher, func() runtime.Object{return &v1beta1.Deployment{}},pendingStages)


    // Watch services
    servicesListWatcher := cache.NewListWatchFromClient(
        executor.client.CoreV1().RESTClient(),
        "services", namespace, fields.Everything())
    servObserver := NewKubernetesObserver(servicesListWatcher, func() runtime.Object{return &v1.Service{}},pendingStages)

    return &KubernetesController{
        deployments: depObserver,
        services: servObserver,
        pendingStages: pendingStages,
    }
}

// Add a resource to be monitored indicating its id on the target platform (uid) and the stage identifier.
func (c *KubernetesController) AddMonitoredResource(uid string, stageId string) {
    c.pendingStages.AddResource(uid,stageId)
}

// Run this controller with its corresponding observers
func (c *KubernetesController) Run() {
    log.Debug().Msgf("time to run K8s controller")
    // Run services controller
    go c.services.Run(1)
    // Run deployments controller
    go c.deployments.Run(1)
}

func (c *KubernetesController) Stop() {
    defer close(c.deployments.stopCh)
    defer close(c.services.stopCh)
}


type KubernetesObserver struct {
    indexer  cache.Indexer
    queue    workqueue.RateLimitingInterface
    informer cache.Controller
    pendingChecks *executor.PendingStages
    // channel to control pod stop
    stopCh chan struct{}
}

// Build a new kubernetes observer for an available API resource.
//  params:
//   watcher containing the api resource name to be queried
//   targetInterface name of the class which objects we are going to store
//   checks list of pending stages
//  return:
//   a pointer to a kubernetes observer
//func NewKubernetesObserver (watcher *cache.ListWatch, targetObject runtime.Object, checks *executor.PendingStages) *KubernetesObserver {
func NewKubernetesObserver (watcher *cache.ListWatch, targetFunc func()runtime.Object, checks *executor.PendingStages) *KubernetesObserver {

    // create the workqueue
    queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
    // Bind the workqueue to a cache with the help of an informer. This way we make sure that
    // whenever the cache is updated, the pod key is added to the workqueue.
    // Note that when we finally process the item from the workqueue, we might see a newer version
    // of the Pod than the version which was responsible for triggering the update.

    indexer, informer := cache.NewIndexerInformer(watcher, targetFunc(), 0, cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            key, err := cache.MetaNamespaceKeyFunc(obj)
            if err == nil {
                queue.Add(key)
            }
        },
        UpdateFunc: func(old interface{}, new interface{}) {
            key, err := cache.MetaNamespaceKeyFunc(new)
            if err == nil {
                queue.Add(key)
            }
        },
        DeleteFunc: func(obj interface{}) {
            // IndexerInformer uses a delta queue, therefore for deletes we have to use this
            // key function.
            key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
            if err == nil {
                queue.Add(key)
            }
        },
    }, cache.Indexers{})

    return &KubernetesObserver{
        informer: informer,
        indexer:  indexer,
        queue:    queue,
        pendingChecks: checks,
        stopCh:  make(chan struct{}),
    }
}


func (c *KubernetesObserver) processNextItem() bool {
    // Wait until there is a new item in the working queue
    key, quit := c.queue.Get()
    if quit {
        return false
    }
    // Tell the queue that we are done with processing this key. This unblocks the key for other workers
    // This allows safe parallel processing because two pods with the same key are never processed in
    // parallel.
    defer c.queue.Done(key)

    // Invoke the method containing the business logic
    err := c.updatePendingChecks(key.(string))
    // Handle the error if something went wrong during the execution of the business logic
    c.handleErr(err, key)
    return true
}

// updatePendingChecks is the business logic of the controller. In this controller it simply prints
// information about the pod to stdout. In case an error happened, it has to simply return the error.
// The retry logic should not be part of the business logic.
func (c *KubernetesObserver) updatePendingChecks(key string) error {
    obj, exists, err := c.indexer.GetByKey(key)
    if err != nil {
        log.Error().Msgf("fetching object with key %s from store failed with %v", key, err)
        return err
    }

    if !exists {
        // Below we will warm up our cache with a Pod, so that we will see a delete for one pod
        log.Debug().Msgf("deployment %s does not exist anymore", key)
    } else {
        // Note that you also have to check the uid if you have a local controlled resource, which
        // is dependent on the actual instance, to detect that a Pod was recreated with the same name

        

        switch v:=obj.(type) {
        case v1beta1.Deployment:
            dep := obj.(runtime.Object)
            kind := dep.GetObjectKind()
        case v1beta1.Deployment:
            dep := obj.(*v1beta1.Deployment)
            log.Debug().Msgf("deployment %s status %v", dep.GetName(), dep.Status.String())
            // This deployment is monitored, and all its replicas are available
            if c.pendingChecks.IsMonitoredResource(string(dep.GetUID())) && dep.Status.UnavailableReplicas == 0 {
                c.pendingChecks.RemoveResource(string(dep.GetUID()))
            }
        case v1.Service:
            dep := obj.(*v1.Service)
            log.Debug().Msgf("service %s status %v", dep.GetName(), dep.Status.String())

            // This deployment is monitored, and all its replicas are available
            if c.pendingChecks.IsMonitoredResource(string(dep.GetUID())) {
                c.pendingChecks.RemoveResource(string(dep.GetUID()))
            }
        default:
            log.Error().Msgf("the incoming object has unknown type %s",v)
        }

    }
    return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *KubernetesObserver) handleErr(err error, key interface{}) {
    if err == nil {
        // Forget about the #AddRateLimited history of the key on every successful synchronization.
        // This ensures that future processing of updates for this key is not delayed because of
        // an outdated error history.
        c.queue.Forget(key)
        return
    }

    // This controller retries 5 times if something goes wrong. After that, it stops trying.
    if c.queue.NumRequeues(key) < 5 {
        log.Error().Msgf("Error syncing pod %v: %v", key, err)

        // Re-enqueue the key rate limited. Based on the rate limiter on the
        // queue and the re-enqueue history, the key will be processed later again.
        c.queue.AddRateLimited(key)
        return
    }

    c.queue.Forget(key)
    // Report to an external entity that, even after several retries, we could not successfully process this key
    utilruntime.HandleError(err)
    log.Debug().Msgf("Dropping pod %q out of the queue: %v", key, err)
}

func (c *KubernetesObserver) Run(threadiness int) {
    defer utilruntime.HandleCrash()

    // Let the workers stop when we are done
    defer c.queue.ShutDown()
    log.Debug().Msg("starting Pod controller")

    go c.informer.Run(c.stopCh)

    // Wait for all involved caches to be synced, before processing items from the queue is started
    if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
        utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
        return
    }

    for i := 0; i < threadiness; i++ {
        go wait.Until(c.runWorker, time.Second, c.stopCh)
    }

    <-c.stopCh
    glog.Info("Stopping Pod controller")
}

func (c *KubernetesObserver) runWorker() {
    for c.processNextItem() {
    }
}


