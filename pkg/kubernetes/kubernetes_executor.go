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
    Client *kubernetes.Clientset
}

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
    toReturn := KubernetesExecutor{c}
    return &toReturn, err
}

// Execute a fragment of a deployment plan for the kubernetes platform.
// Every service defined into the stage is translated into k8s and executed. Then, the controller monitors the
// correct completion of the deployment if the deployment fails, a rollback operation terminating all the services
// is done.
func (k *KubernetesExecutor) Execute(fragment *pbConductor.DeploymentFragment, stage *pbConductor.DeploymentStage) (*executor.Deployable,error) {
    log.Info().Str("stage",stage.StageId).Msgf("execute stage %s with %d services", stage.StageId, len(stage.Services))

    targetNamespace := getNamespace(fragment.AppId)

    var resources executor.Deployable
    // Build the structures to be executed. If any of then cannot be built, we have a failure.
    k8sDeploy := NewDeployableKubernetesStage(k.Client, stage, targetNamespace)
    resources = k8sDeploy

    err := k8sDeploy.Build()

    if err != nil {
        log.Error().Err(err).Msgf("impossible to build resources for stage %s in fragment %s",stage.StageId, stage.FragmentId)
        return &resources,err
    }

    // Build a controller for this deploy operation
    checks := executor.NewPendingStages()
    kontroller := NewKubernetesController(k, checks, targetNamespace)

    var k8sController *KubernetesController
    k8sController = kontroller.(*KubernetesController)

    // Deploy everything and then start the controller.
    err = k8sDeploy.Deploy(kontroller)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to deploy resources for stage %s in fragment %s",stage.StageId, stage.FragmentId)
        return &resources,err
    }

    // run the controller
    k8sController.Run()

    // TODO supervise that the deployment for this stage was correct
    stageErr := k.checkPendingStage(checks, stage)
    k8sController.Stop()
    return &resources,stageErr
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
func(k *KubernetesExecutor) checkPendingStage(checks *executor.PendingStages, stage *pbConductor.DeploymentStage) error {
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
            pending := checks.HasPendingChecks(stage.StageId)
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

    deploymentsClient := k.Client.AppsV1().Deployments(apiv1.NamespaceDefault)
    err := deploymentsClient.Delete(serv.Name, metav1.NewDeleteOptions(2000))
    if err != nil {
        log.Error().Err(err).Msgf("problems deleting service %s", serv.Name)
        return err
    }

    return nil
}

func(k *KubernetesExecutor) UndeployResources(dep DeployableKubernetesStage) error {
    //TODO create interface for resources and force every resource to have a create/deploy/undeploy function and how to watch
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

