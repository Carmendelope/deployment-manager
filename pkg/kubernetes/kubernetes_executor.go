/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbApplication "github.com/nalej/grpc-application-go"
    "github.com/nalej/deployment-manager/pkg/executor"
    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apiv1 "k8s.io/api/core/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "github.com/rs/zerolog/log"
    "os"
    "flag"
    "path/filepath"
    "errors"
    "fmt"
    "time"
)


const (
    // Time in seconds we wait for a stage to be finished.
    StageCheckingTimeout = 30
    // Time between pending stage checks in seconds
    CheckingSleepTime = 1
)

// Description of resources that can be deployed into k8s. This structure is used to encapsulate



// The executor is the main structure in charge of running deployment plans on top of K8s.
type KubernetesExecutor struct {
    client *kubernetes.Clientset
    pendingStages *PendingStages
}

//func NewKubernetesExecutor(internal bool, pendingChecks *PendingChecks) (executor.Executor,error) {
func NewKubernetesExecutor(internal bool) (executor.Executor,error) {
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
    pendingChecks := NewPendingStages()
    toReturn := KubernetesExecutor{c, pendingChecks}
    return &toReturn, err
}

// Execute a fragment of a deployment plan for the kubernetes platform.
// Every service defined into the stage is translated into k8s and executed. Then, the controller monitors the
// correct completion of the deployment if the deployment fails, a rollback operation terminating all the services
// is done.
func (k *KubernetesExecutor) Execute(fragment *pbConductor.DeploymentFragment, stage *pbConductor.DeploymentStage) error {
    log.Info().Str("stage",stage.StageId).Msgf("execute stage %s with %d services", stage.StageId, len(stage.Services))

    // Build the structures to be executed. If any of then cannot be built, we have a failure.
    resources, err := BuildResources(fragment, stage)

    if err != nil {
        log.Error().Err(err).Msgf("impossible to create deployables for stage %s in deployment %s", stage.StageId, fragment.FragmentId)
        return err
    }
    // create namespace
    err = k.createNamespace(stage.StageId,resources.targetNamespace)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to create namespace %s for stageId %s", resources.targetNamespace, stage.StageId)
        return err
    }
    // create deployments
    err = k.createDeployments(stage.StageId,resources.targetNamespace, resources.deployments)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to create deployments for stageId %s", stage.StageId)
        return err
    }

    // create services
    err = k.createServices(stage.StageId, resources.targetNamespace, resources.services)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to create services for stageId %s", stage.StageId)
        return err
    }

    // TODO supervise that the deployment for this stage was correct
    stageErr := k.checkPendingStage(stage)
    return stageErr
}


// Create a new namespace
//  params:
//   stageId id this deployment belongs to
//   namespace string for the name of this namespace
//  return:
//   error if any
func (k *KubernetesExecutor) createNamespace(stageId string, namespace string) error {
    // TODO check if namespace already exists
    n := apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
    c := k.client.CoreV1().Namespaces()
    _, err := c.Create(&n)
    return err
}

// Create new deployments
//  params:
//   stageId id this deployment belongs to
//   namespace string for the name of this namespace
//   deployments list of deployments to run
//  return:
//   error if any
func (k *KubernetesExecutor) createDeployments(stageId string, namespace string, deployments []appsv1.Deployment) error {
    deploymentsClient := k.client.AppsV1().Deployments(namespace)
    for _, dep := range deployments {
        deployed, err := deploymentsClient.Create(&dep)
        if err != nil {
            log.Error().Err(err).Msgf("error creating deployment %s",dep.Name)
            return err
        }
        // Add to the list of pending checks
        k.pendingStages.AddResource(string(deployed.GetUID()), stageId)
    }
    return nil
}

// Create new services
//  params:
//   stageId id this deployment belongs to
//   namespace string for the name of this namespace
//   services list of services to run
//  return:
//   error if any
func (k *KubernetesExecutor) createServices(stageId string, namespace string, services []apiv1.Service) error {
    servicesClient := k.client.CoreV1().Services(namespace)
    for _, serv := range services {
        _, err := servicesClient.Create(&serv)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service %s",serv.Name)
            return err
        }
    }
    return nil
}



func (k *KubernetesExecutor) StageRollback(stage *pbConductor.DeploymentStage) error {
    log.Info().Msgf("running rollback operation for deployment stage %s from fragment plan %s",stage.StageId, stage.FragmentId)
    for _, serv := range stage.Services {
        err := k.UndeployService(serv)
        if err != nil {
            log.Error().Err(err).Msgf("error undeploying service %s from stage %s",serv.ServiceId, stage.StageId)
            return err
        }
    }
    return nil
}



// Check iteratively if the stage has any pending resource to be deployed. This is done using the kubernetes controller.
// If after the maximum expiration time the check is not successful, the execution is considered to be failed.
func(k *KubernetesExecutor) checkPendingStage(stage *pbConductor.DeploymentStage) error {
    log.Info().Msgf("stage %s wait until all stages are complete",stage.StageId)
    timeout := time.After(time.Second * StageCheckingTimeout)
    tick := time.Tick(time.Second * CheckingSleepTime)
    for {
        select {
        // Got a timeout! Error
        case <-timeout:
            log.Error().Msgf("checking pending resources exceeded for stage %s", stage.StageId)
            return errors.New(fmt.Sprintf("checking pending resources exceeded for stage %s", stage.StageId))
        // Next check
        case <-tick:
            pending := k.pendingStages.HasPendingChecks(stage.StageId)
            if !pending {
                log.Info().Msgf("stage %s has no pending checks. Exit checking stage", stage.StageId)
                return nil
            }
        }
    }
}


// Undeploy a service if running.
//  params:
//   serv service to be removed
//  return:
//   error if any
func(k *KubernetesExecutor) UndeployService(serv *pbApplication.Service) error {
    if serv == nil {
        returnError := errors.New("nil service was requested to be undeployed")
        log.Error().Err(returnError).Msg("impossible to undeploy nil instance")
        return returnError
    }

    deploymentsClient := k.client.AppsV1().Deployments(apiv1.NamespaceDefault)
    err := deploymentsClient.Delete(serv.Name, metav1.NewDeleteOptions(2000))
    if err != nil {
        log.Error().Err(err).Msgf("problems deleting service %s", serv.Name)
        return err
    }

    return nil
}

func(k *KubernetesExecutor) UndeployResources(dep DeployableResources) error {
    //TODO create interface for resources and force every resource to have a create/deploy/undeploy function and how to watch
    return nil
}


// Create a new kubernetes client using deployment inside the cluster.
//  params:
//   internal true if the client is deployed inside the cluster.
//  return:
//   instance for the k8s client or error if any
func getInternalKubernetesClient() (*kubernetes.Clientset,error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        log.Panic().Err(err).Msg("impossible to get local configuration for internal k8s client")
        return nil, err
    }
    // creates the clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Panic().Err(err).Msg("impossible to instantiate k8s client")
        return nil, err
    }
    return clientset,nil
}


// Create a new kubernetes client using deployment outside the cluster.
//  params:
//   internal true if the client is deployed inside the cluster.
//  return:
//   instance for the k8s client or error if any
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

