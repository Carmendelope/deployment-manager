/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
	"errors"
	"flag"
	"github.com/nalej/deployment-manager/pkg"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/monitor"
	pbConductor "github.com/nalej/grpc-conductor-go"
	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)




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
    toReturn := KubernetesExecutor{Client: c}
    return &toReturn, err
}

func(k *KubernetesExecutor) BuildNativeDeployable(stage *pbConductor.DeploymentStage, namespace string, ztNetworkId string,
    organizationId string, organizationName string, deploymentId string, appInstanceId string, appName string,
    clusterPublicHostname string, dnsHosts []string) (executor.Deployable, error){

    log.Debug().Msgf("fragment %s stage %s requested to be translated into K8s deployable",
        stage.FragmentId, stage.StageId)

    var resources executor.Deployable
    // Build the structures to be executed. If any of then cannot be built, we have a failure.
    k8sDeploy := NewDeployableKubernetesStage(k.Client, stage, namespace, ztNetworkId, organizationId,
        organizationName, deploymentId, appInstanceId, appName, clusterPublicHostname, dnsHosts)
    resources = k8sDeploy

    err := k8sDeploy.Build()

    if err != nil {
        log.Error().Err(err).Msgf("impossible to build resources for stage %s in fragment %s",stage.StageId, stage.FragmentId)
        return nil,err
    }

    return resources, nil
}

// Prepare the namespace for the deployment. This is a special case because all the deployments will share a common
// namespace. If this step cannot be done, no stage deployment will start.
func (k *KubernetesExecutor) PrepareEnvironmentForDeployment(fragment *pbConductor.DeploymentFragment, namespace string,
    monitor *monitor.MonitorHelper) (executor.Deployable, error) {
    // Create a namespace
    namespaceDeployable := NewDeployableNamespace(k.Client, fragment.FragmentId, namespace)
    err := namespaceDeployable.Build()
    if err != nil {
       log.Error().Err(err).Msgf("impossible to build namespace %s",namespace)
       return nil, err
    }

    // Prepare a controller to be sure that we have a working namespace
    // Build a controller for this deploy operation
    checks := executor.NewPendingStages(fragment.OrganizationId,fragment.AppInstanceId,fragment.FragmentId, monitor)
    // Second, instantiate a new controller
    kontroller := NewKubernetesController(k, checks, namespaceDeployable.targetNamespace)

    err = namespaceDeployable.Deploy(kontroller)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to deploy namespace %s",namespace)
        return nil,err
    }

    var toReturn executor.Deployable
    toReturn = namespaceDeployable

    return toReturn, nil
}


// Deploy a stage into kubernetes. This function
func (k *KubernetesExecutor) DeployStage(toDeploy executor.Deployable, fragment *pbConductor.DeploymentFragment,
    stage *pbConductor.DeploymentStage, monitor *monitor.MonitorHelper) error {
    log.Info().Str("stage",stage.StageId).Msgf("execute stage %s with %d services", stage.StageId, len(stage.Services))

    var k8sDeployable *DeployableKubernetesStage
    k8sDeployable = toDeploy.(*DeployableKubernetesStage)


    // Build a controller for this deploy operation
    // First, build a struct in charge of the pending stages
    checks := executor.NewPendingStages(fragment.OrganizationId,fragment.AppInstanceId,fragment.FragmentId, monitor)
    // Second, instantiate a new controller
    kontroller := NewKubernetesController(k, checks, k8sDeployable.targetNamespace)

    var k8sController *KubernetesController
    k8sController = kontroller.(*KubernetesController)

    // Deploy everything and then start the controller.
    err := toDeploy.Deploy(kontroller)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to deploy resources for stage %s in fragment %s",stage.StageId, stage.FragmentId)
        return err
    }

    // run the controller
    k8sController.Run()
    stageErr := checks.WaitPendingChecks(stage.StageId)
    k8sController.Stop()
    return stageErr

}

// Internal function to execute a given set of deployable items.
func (k *KubernetesExecutor) runStage(targetNamespace string, toDeploy executor.Deployable,
    fragment *pbConductor.DeploymentFragment, stage *pbConductor.DeploymentStage,monitor *monitor.MonitorHelper) error {

    // Build a controller for this deploy operation
    // First, build a struct in charge of the pending stages
    checks := executor.NewPendingStages(fragment.OrganizationId,fragment.AppInstanceId,fragment.FragmentId, monitor)
    // Second, instantiate a new controller
    kontroller := NewKubernetesController(k, checks, targetNamespace)

    var k8sController *KubernetesController
    k8sController = kontroller.(*KubernetesController)

    // Deploy everything and then start the controller.
    err := toDeploy.Deploy(kontroller)
    if err != nil {
        log.Error().Err(err).Msgf("impossible to deploy resources for stage %s in fragment %s",stage.StageId, stage.FragmentId)
        return err
    }

    // run the controller
    k8sController.Run()
    stageErr := checks.WaitPendingChecks(stage.StageId)
    k8sController.Stop()
    return stageErr
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
    targetNS := pkg.GetNamespace(request.OrganizationId, request.AppInstanceId)
    log.Info().Str("app_instance_id", request.AppInstanceId).Str("targetNS", targetNS).Msg("undeploy app namespace")

    ns := NewDeployableNamespace(k.Client, "not necessary", targetNS)
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

