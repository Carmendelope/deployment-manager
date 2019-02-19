/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
    "github.com/nalej/deployment-manager/internal/entities"
    pbConductor "github.com/nalej/grpc-conductor-go"
    apiv1 "k8s.io/api/core/v1"
    "github.com/rs/zerolog/log"
    "k8s.io/apimachinery/pkg/util/intstr"
    "k8s.io/client-go/kubernetes"
    "github.com/nalej/deployment-manager/pkg/executor"
)

/*
 * Specification of potential k8s deployable resources and their functions.
 */

const (
    // Grace period in seconds to delete a deployable.
    DeleteGracePeriod = 10
)



// Definition of a collection of deployable resources contained by a stage. This object is deployable and it has
// deployable objects itself. The deploy of this object simply consists on the deployment of the internal objects.
type DeployableKubernetesStage struct {
    // kubernetes Client
    client *kubernetes.Clientset
    // deployment data
    data entities.DeploymentMetadata
    // collection of Deployments
    Deployments *DeployableDeployments
    // collection of Services
    Services *DeployableServices
    // Collection of Ingresses to be deployed
    Ingresses *DeployableIngress
    // Collection of maps to be deployed.
    Configmaps *DeployableConfigMaps
    // Collection of Secrets to be deployed.
    Secrets * DeployableSecrets
    // Collection of persistence Volume claims
    Storage *DeployableStorage
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   Client k8s api Client
//   stage these resources belong to
//   targetNamespace name of the namespace the resources will be deployed into
func NewDeployableKubernetesStage (
    client *kubernetes.Clientset, planetPath string, data entities.DeploymentMetadata) *DeployableKubernetesStage {
    return &DeployableKubernetesStage{
        client:          client,
        data:            data,
        Services:        NewDeployableService(client, data),
        Deployments: NewDeployableDeployment(client, data),
        Ingresses:  NewDeployableIngress(client, data),
        Configmaps: NewDeployableConfigMaps(client, data),
        Secrets:    NewDeployableSecrets(client, planetPath, data),
        Storage:    NewDeployableStorage(client, data),
    }
}

func(d DeployableKubernetesStage) GetId() string {
    return d.data.Stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
    // Build Deployments
    err := d.Deployments.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create Deployments")
        return err
    }
    // Build Services
    err = d.Services.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create Services for")
        return err
    }

    err = d.Ingresses.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create Ingresses")
        return err
    }

    err = d.Configmaps.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create Configmaps")
        return err
    }

    err = d.Secrets.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("cannot create Secrets")
        return err
    }

    // Build storage
    err = d.Storage.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.data.Stage.StageId).Msg("impossible to create storage for")
        return err
    }
    return nil
}

func (d DeployableKubernetesStage) Deploy(controller executor.DeploymentController) error {

    // Deploy Secrets
    log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Secrets")
    err := d.Secrets.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Secrets, aborting")
        return err
    }

    // Deploy Configmaps
    log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Configmaps")
    err = d.Configmaps.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Configmaps, aborting")
        return err
    }

    // Deploy Storage
    log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Storage")
    err = d.Storage.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Storage, aborting")
        return err
    }

    // Deploy Deployments
    log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Deployments")
    err = d.Deployments.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Deployments, aborting")
        return err
    }
    // Deploy Services
    log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Services")
    err = d.Services.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Services, aborting")
        return err
    }

    log.Debug().Str("stageId", d.data.Stage.StageId).Msg("Deploy Ingresses")
    err = d.Ingresses.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Ingresses, aborting")
        return err
    }

    return nil
}

func (d DeployableKubernetesStage) Undeploy() error {
    // Deploying the namespace should be enough
    // Deploy namespace
    /*
    err := d.namespace.Undeploy()
    if err != nil {
        return err
    }
    */
    // Deploy Deployments
    err := d.Deployments.Undeploy()
    if err != nil {
        return err
    }
    // Deploy Services
    err = d.Services.Undeploy()
    if err != nil {
        return err
    }
    err = d.Ingresses.Undeploy()
    if err != nil {
        return err
    }
    err = d.Configmaps.Undeploy()
    if err != nil {
        return err
    }
    err = d.Secrets.Undeploy()
    if err != nil {
        return err
    }

    err = d.Storage.Undeploy()
    if err != nil {
        return err
    }
    return nil
}

func getServicePorts(ports []*pbConductor.Port) []apiv1.ServicePort {
    obtained := make([]apiv1.ServicePort, 0, len(ports))
    for _, p := range ports {
        obtained = append(obtained, apiv1.ServicePort{
            Name: p.Name,
            Port: p.ExposedPort,
            TargetPort: intstr.IntOrString{IntVal: p.InternalPort},
        })
    }
    if len(obtained) == 0 {
        return nil
    }
    return obtained
}




