/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    "errors"
    "flag"
    "fmt"
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/executor"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "github.com/rs/zerolog/log"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "os"
    "path/filepath"
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
        PlanetPath:planetPath,
    }
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

    controller, found := k.Controllers[metadata.FragmentId]
    if !found {
        log.Error().Str("fragmentId",metadata.FragmentId).
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
    } else {
        log.Debug().Str("namespace", metadata.Namespace).Msg("namespace already exists... skip")
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
    controller, found := k.Controllers[k8sDeploy.data.FragmentId]
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




func (k *KubernetesExecutor) AddEventsController(fragmentId string, monitored monitor.MonitoredInstances,
    namespace string) executor.DeploymentController {
    // Instantiate a new controller
    deployController := NewKubernetesController(k, monitored, namespace)

    var k8sController *KubernetesController
    k8sController = deployController.(*KubernetesController)

    k.mu.Lock()
    defer k.mu.Unlock()
    retrievedController, found := k.Controllers[fragmentId]
    if found {
        log.Warn().Str("fragmentId", fragmentId).Msg("a kubernetes controller already exists for this namespace")
        return retrievedController
    }
    k.Controllers[fragmentId] = k8sController
    log.Debug().Interface("controllers", k.Controllers).Msg("added a new events controller")
    return k8sController
}


// Generate a events controller for a given namespace.
//  params:
//   fragmentId to be supervised
//   monitored data structure to monitor incoming events
func (k *KubernetesExecutor) StartControlEvents(fragmentId string) executor.DeploymentController {
    k.mu.Lock()
    defer k.mu.Unlock()
    toReturn, found := k.Controllers[fragmentId]
    if !found {
        log.Error().Str("fragmentId",fragmentId).Msg("the kubernetes controller was not found")
        return nil
    }
    toReturn.Run()
    return toReturn
}

// Stop the control of events for a given namespace.
//  params:
//   appInstanceId to stop the control
func (k *KubernetesExecutor) StopControlEvents(fragmentId string) {
    k.mu.Lock()
    defer k.mu.Unlock()
    log.Info().Str("fragmentId", fragmentId).Interface("controllers",k.Controllers).Msg("stop events controller")
    // TODO this is not required as K8s stops the channels
    //controller, found := k.Controllers[appInstanceId]
    //if !found {
    //    log.Error().Str("appInstanceId",appInstanceId).Msg("impossible to stop controller, appInstanceId not found")
    //}
    //controller.Stop()
    // delete the instance
    delete(k.Controllers,fragmentId)
}


func (k *KubernetesExecutor) UndeployStage(stage *pbConductor.DeploymentStage, toUndeploy executor.Deployable) error {
    log.Info().Msgf("undeploy stage %s from fragment %s", stage.StageId, stage.FragmentId)
    err := toUndeploy.Undeploy()
    if err != nil {
        log.Error().Msgf("error undeploying stage %s from fragment %s", stage.StageId, stage.FragmentId)
    }
    return err
}

// TODO reorganize this function to use balance the code among the different deployable entities
func (k *KubernetesExecutor) UndeployFragment(namespace string, fragmentId string) error {
    log.Info().Msgf("undeploy fragment %s in namespace %s", fragmentId, namespace)

    deleteOptions := metav1.DeleteOptions{}
    queryOptions := metav1.ListOptions{LabelSelector: fmt.Sprintf("nalej-deployment-fragment=%s", fragmentId)}

    // deployments
    err := k.Client.AppsV1().Deployments(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // replica sets
    err = k.Client.AppsV1().ReplicaSets(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // config maps
    err = k.Client.CoreV1().ConfigMaps(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // ingress
    err = k.Client.CoreV1().Endpoints(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // load balancers
    err = k.Client.CoreV1().PersistentVolumeClaims(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // secrets
    err = k.Client.CoreV1().Secrets(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // jobs
    err = k.Client.BatchV1().Jobs(namespace).DeleteCollection(&deleteOptions, queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    // services
    list, err := k.Client.CoreV1().Services(namespace).List(queryOptions)
    if err != nil {
        log.Error().Err(err).Msg("error undeploying fragments")
    }
    for _, x := range list.Items {
        err = k.Client.CoreV1().Services(namespace).Delete(x.Name, &deleteOptions)
        if err != nil {
            log.Error().Err(err).Msg("error undeploying fragments")
        }
    }

    return nil
}


func (k *KubernetesExecutor) UndeployNamespace(request *pbDeploymentMgr.UndeployRequest) error {
    // TODO check if this operation can remove iterative namespaces 0-XXXXX, 1-XXXXX, etc
    // A partially filled namespace object should be enough to find the target namespace
    metadata := entities.DeploymentMetadata{OrganizationId: request.OrganizationId, AppInstanceId: request.AppInstanceId}

    ns := NewDeployableNamespace(k.Client, metadata)
    err := ns.Build()
    if err != nil {
        log.Error().Msgf("error building deployable namespace %s", ns.namespace.Name)
        return err
    }

    err = ns.Undeploy()
    if err != nil {
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

