/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
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
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // namespace name descriptor
    targetNamespace string
    // ZeroTier network id
    ztNetworkId string
    // collection of deployments
    deployments *DeployableDeployments
    // collection of services
    services *DeployableServices
    // Collection of ingresses to be deployed
    ingresses *DeployableIngress
    // Collection of maps to be deployed.
    configmaps *DeployableConfigMaps
    // Collection of secrets to be deployed.
    secrets * DeployableSecrets
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   Client k8s api Client
//   stage these resources belong to
//   targetNamespace name of the namespace the resources will be deployed into
func NewDeployableKubernetesStage (client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    targetNamespace string, ztNetworkId string, organizationId string, organizationName string,
    deploymentId string, appInstanceId string, appName string, clusterPublicHostname string, dnsHosts []string) *DeployableKubernetesStage {
    return &DeployableKubernetesStage{
        client: client,
        stage: stage,
        targetNamespace: targetNamespace,
        ztNetworkId: ztNetworkId,
        services: NewDeployableService(client, stage, targetNamespace),
        deployments: NewDeployableDeployment(client, stage, targetNamespace,ztNetworkId, organizationId,
            organizationName, deploymentId, appInstanceId, appName, dnsHosts),
        ingresses: NewDeployableIngress(client, appInstanceId, stage, targetNamespace, clusterPublicHostname),
        configmaps: NewDeployableConfigMaps(client, stage, targetNamespace),
        secrets: NewDeployableSecrets(client, stage, targetNamespace),
    }
}

func(d DeployableKubernetesStage) GetId() string {
    return d.stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
    // Build deployments
    err := d.deployments.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("impossible to create deployments")
        return err
    }
    // Build services
    err = d.services.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("impossible to create services for")
        return err
    }

    err = d.ingresses.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create ingresses")
        return err
    }

    err = d.configmaps.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create configmaps")
        return err
    }

    err = d.secrets.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create secrets")
        return err
    }

    return nil
}

func (d DeployableKubernetesStage) Deploy(controller executor.DeploymentController) error {

    // Deploy secrets
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy secrets")
    err := d.secrets.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying secrets, aborting")
        return err
    }

    // Deploy configmaps
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy configmaps")
    err = d.configmaps.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying configmaps, aborting")
        return err
    }

    // Deploy deployments
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy deployments")
    err = d.deployments.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying deployments, aborting")
        return err
    }
    // Deploy services
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy services")
    err = d.services.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying services, aborting")
        return err
    }

    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy ingresses")
    err = d.ingresses.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying ingresses, aborting")
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
    // Deploy deployments
    err := d.deployments.Undeploy()
    if err != nil {
        return err
    }
    // Deploy services
    err = d.services.Undeploy()
    if err != nil {
        return err
    }
    err = d.ingresses.Undeploy()
    if err != nil {
        return err
    }
    err = d.configmaps.Undeploy()
    if err != nil {
        return err
    }
    err = d.secrets.Undeploy()
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




