/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
	"errors"
	"flag"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/executor"
	pbConductor "github.com/nalej/grpc-conductor-go"
	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
    "github.com/nalej/deployment-manager/pkg/common"
    "sync"
)


// The executor is the main structure in charge of running deployment plans on top of K8s.
type KubernetesExecutor struct {
    Client *kubernetes.Clientset
    // Map of controllers
    // Namespace -> controller
    Controllers map[string]*KubernetesController
    PlanetPath string
    // mutex
    mu sync.Mutex
}

func NewKubernetesExecutor(internal bool, planetPath string) (executor.Executor,error) {
    var c *kubernetes.Clientset
    var err error

    if internal {
        c, err = getInternalKubernetesClient()
    } else {
        c, err = getExternalKubernetesClient()
    }
    if err!=nil{
        log.Error().Err(err).Msg("impossible to create kubernetes clientset")
        return nil, err
    }
    if c==nil{
        foundError := errors.New("kubernetes clientset was nil")
        log.Error().Err(foundError)
        return nil, foundError
    }
    toReturn := KubernetesExecutor{
        Client: c,
        Controllers: make(map[string]*KubernetesController,0),
        PlanetPath:planetPath}
    return &toReturn, err
}

func(k *KubernetesExecutor) BuildNativeDeployable(metadata entities.DeploymentMetadata) (executor.Deployable, error){

    log.Debug().Msgf("fragment %s stage %s requested to be translated into K8s deployable",
        metadata.FragmentId, metadata.Stage.StageId)

    var resources executor.Deployable
    k8sDeploy := NewDeployableKubernetesStage(k.Client, k.PlanetPath, metadata)
    resources = k8sDeploy

    err := k8sDeploy.Build()

    if err != nil {
        log.Error().Err(err).Msgf("impossible to build resources for stage %s in fragment %s",
            metadata.Stage.StageId, metadata.FragmentId)
        return nil,err
    }

    log.Debug().Interface("metadata",metadata).Interface("k8sDeployable",resources).Msg("built k8s deployable")

    return resources, nil
}

// Prepare the namespace for the deployment. This is a special case because all the Deployments will share a common
// namespace. If this step cannot be done, no stage deployment will start.
func (k *KubernetesExecutor) PrepareEnvironmentForDeployment(metadata entities.DeploymentMetadata)(executor.Deployable, error) {
    log.Debug().Str("fragmentId",metadata.FragmentId).Msg("prepare environment for deployment")

    // Create a namespace
    namespaceDeployable := NewDeployableNamespace(k.Client, metadata)
    err := namespaceDeployable.Build()
    if err != nil {
       log.Error().Err(err).Msgf("impossible to build namespace %s",metadata.Namespace)
       return nil, err
    }

    controller, found := k.Controllers[metadata.Namespace]
    if !found {
        log.Error().Str("namespace",metadata.Namespace).
            Msg("impossible to find the corresponding events controller")
        return nil, errors.New("impossible to find the corresponding events controller")
    }

    if !namespaceDeployable.exists() {
        log.Debug().Str("namespace", metadata.Namespace).Msg("create namespace...")
        // TODO Check if namespace already exists...
        err = namespaceDeployable.Deploy(controller)
        if err != nil {
            log.Error().Err(err).Msgf("impossible to deploy namespace %s",metadata.Namespace)
            return nil,err
        }
        log.Debug().Str("namespace", metadata.Namespace).Msg("namespace... created")

        // NP-766. create the nalej-public-registry on the user namespace
        nalejSecret := NewDeployableNalejSecret(k.Client, metadata)
        err = nalejSecret.Build()
        if err != nil {
            log.Error().Err(err).Msg("impossible to build nalej-public-registry secret")
        }
        err = nalejSecret.Deploy(controller)
        if err != nil {
            log.Error().Err(err).Msg("impossible to deploy nalej-public-registry secret")
            return nil,err
        }
    }

    var toReturn executor.Deployable
    toReturn = namespaceDeployable


    return toReturn, nil
}


// Deploy a stage into kubernetes. This function
func (k *KubernetesExecutor) DeployStage(toDeploy executor.Deployable, fragment *pbConductor.DeploymentFragment,
    stage *pbConductor.DeploymentStage, monitoredInstances monitor.MonitoredInstances) error {
    log.Info().Str("stage",stage.StageId).Msgf("execute stage %s with %d Services", stage.StageId, len(stage.Services))

    var k8sDeploy *DeployableKubernetesStage
    k8sDeploy = toDeploy.(*DeployableKubernetesStage)

    // get the previously generated controller for this namespace
    controller, found := k.Controllers[k8sDeploy.data.Namespace]
    if !found {
        log.Error().Str("namespace",k8sDeploy.data.Namespace).Str("stageId",stage.StageId).
            Msg("impossible to find the corresponding events controller")
        return errors.New("impossible to find the corresponding events controller")
    }

    // Deploy everything and then start the controller.
    err := toDeploy.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to deploy resources for stage %s in fragment %s",stage.StageId, stage.FragmentId)
        return err
    }

    return err

}


func (k *KubernetesExecutor) AddEventsController(namespace string, monitored monitor.MonitoredInstances) executor.DeploymentController {
    // Instantiate a new controller
    deployController := NewKubernetesController(k, monitored, namespace)

    var k8sController *KubernetesController
    k8sController = deployController.(*KubernetesController)

    k.mu.Lock()
    defer k.mu.Unlock()
    retrievedController, found := k.Controllers[namespace]
    if found {
        log.Warn().Str("namespace", namespace).Msg("a kubernetes controller already exists for this namespace")
        return retrievedController
    }
    k.Controllers[namespace] = k8sController
    log.Debug().Interface("controllers", k.Controllers).Msg("added a new events controller")
    return k8sController
}


// Generate a events controller for a given namespace.
//  params:
//   namespace to be supervised
//   monitored data structure to monitor incoming events
func (k *KubernetesExecutor) StartControlEvents(namespace string) executor.DeploymentController {
    k.mu.Lock()
    defer k.mu.Unlock()
    toReturn, found := k.Controllers[namespace]
    if !found {
        log.Error().Str("namespace",namespace).Msg("the kubernetes controller was not found")
        return nil
    }
    toReturn.Run()
    return toReturn
}

// Stop the control of events for a given namespace.
//  params:
//   namespace to stop the control
func (k *KubernetesExecutor) StopControlEvents(namespace string) {
    k.mu.Lock()
    defer k.mu.Unlock()
    log.Info().Str("namespace", namespace).Interface("controllers",k.Controllers).Msg("stop events controller")
    controller, found := k.Controllers[namespace]
    if !found {
        log.Error().Str("namespace",namespace).Msg("impossible to stop controller, namespace not found")
    }
    controller.Stop()
    // delete the instance
    delete(k.Controllers,namespace)
}


func (k *KubernetesExecutor) UndeployStage(stage *pbConductor.DeploymentStage, toUndeploy executor.Deployable) error {
    log.Info().Msgf("undeploy stage %s from fragment %s", stage.StageId, stage.FragmentId)
    err := toUndeploy.Undeploy()
    if err != nil {
        log.Error().Msgf("error undeploying stage %s from fragment %s", stage.StageId, stage.FragmentId)
    }
    return err
}


func (k *KubernetesExecutor) UndeployFragment(fragment *pbConductor.DeploymentStage, toUndeploy executor.Deployable) error {
    log.Info().Msgf("undeploy fragment %s", fragment.FragmentId)
    err := toUndeploy.Undeploy()
    if err != nil {
        log.Error().Msgf("error undeploying fragment %s", fragment.FragmentId)
    }
    return err
}

func (k *KubernetesExecutor) UndeployNamespace(request *pbDeploymentMgr.UndeployRequest) error {
    targetNS := common.GetNamespace(request.OrganizationId, request.AppInstanceId)
    log.Info().Str("app_instance_id", request.AppInstanceId).Str("targetNS", targetNS).Msg("undeploy app namespace")

    // partially fill a deployment metadata entry with the target namespace
    metadata := entities.DeploymentMetadata{Namespace: targetNS}

    ns := NewDeployableNamespace(k.Client, metadata)
    err := ns.Build()
    if err != nil {
        log.Error().Msgf("error building deployable namespace %s", ns.namespace.Name)
        return err
    }

    err = ns.Undeploy()
    if err != nil {
        log.Error().Msgf("error undeploying application %s in namespace %s", request.AppInstanceId, ns.namespace.Name)
        return err
    }


    return nil
}

func (k *KubernetesExecutor) StageRollback(stage *pbConductor.DeploymentStage, deployed executor.Deployable) error {
    log.Info().Msgf("requested rollback for stage %s",stage.StageId)
    // Call the undeploy for this deployable
    err := deployed.Undeploy()
    if err != nil {
        return err
    }

    return nil
}


// Create a new kubernetes Client using deployment inside the cluster.
//  params:
//   internal true if the Client is deployed inside the cluster.
//  return:
//   instance for the k8s Client or error if any
func getInternalKubernetesClient() (*kubernetes.Clientset,error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        log.Panic().Err(err).Msg("impossible to get local configuration for internal k8s Client")
        return nil, err
    }
    // creates the clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Panic().Err(err).Msg("impossible to instantiate k8s Client")
        return nil, err
    }
    return clientset,nil
}


// Create a new kubernetes Client using deployment outside the cluster.
//  params:
//   internal true if the Client is deployed inside the cluster.
//  return:
//   instance for the k8s Client or error if any
func getExternalKubernetesClient() (*kubernetes.Clientset,error) {
    var kubeconfig *string
    if home := homeDir(); home != "" {
        kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
    } else {
        kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
    }
    flag.Parse()

    // use the current context in kubeconfig
    config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
    if err != nil {
        log.Panic().Err(err).Msg("error building configuration from kubeconfig")
        return nil, err
    }

    // create the clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Panic().Err(err).Msg("error using configuration to build k8s clientset")
        return nil, err
    }

    return clientset, nil
}

func homeDir() string {
    if h := os.Getenv("HOME"); h != "" {
        return h
    }
    return os.Getenv("USERPROFILE") // windows
}


// Helping function for pointer conversion.
func int32Ptr(i int32) *int32 { return &i }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b}

