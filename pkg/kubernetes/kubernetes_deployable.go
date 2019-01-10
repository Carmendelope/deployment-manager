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
    // Nalej defined variables
    nalejVariables map[string]string
    // ZeroTier network id
    ztNetworkId string
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
}

// Instantiate a new set of resources for a stage to be deployed.
//  params:
//   Client k8s api Client
//   stage these resources belong to
//   targetNamespace name of the namespace the resources will be deployed into
func NewDeployableKubernetesStage (
    client *kubernetes.Clientset, stage *pbConductor.DeploymentStage, targetNamespace string,
    nalejVariables map[string]string, ztNetworkId string, organizationId string,
    organizationName string, deploymentId string, appInstanceId string,
    appName string, clusterPublicHostname string, dnsHosts []string) *DeployableKubernetesStage {
    return &DeployableKubernetesStage{
        client:          client,
        stage:           stage,
        targetNamespace: targetNamespace,
        nalejVariables:  nalejVariables,
        ztNetworkId:     ztNetworkId,
        Services:        NewDeployableService(client, stage, targetNamespace),
        Deployments: NewDeployableDeployment(
                        client, stage, targetNamespace, nalejVariables, ztNetworkId, organizationId,
                        organizationName, deploymentId, appInstanceId, appName, dnsHosts),
        Ingresses:  NewDeployableIngress(client, appInstanceId, stage, targetNamespace, clusterPublicHostname),
        Configmaps: NewDeployableConfigMaps(client, stage, targetNamespace),
        Secrets:    NewDeployableSecrets(client, stage, targetNamespace),
    }
}

func(d DeployableKubernetesStage) GetId() string {
    return d.stage.StageId
}

func (d DeployableKubernetesStage) Build() error {
    // Build Deployments
    err := d.Deployments.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("impossible to create Deployments")
        return err
    }
    // Build Services
    err = d.Services.Build()
    if err != nil {
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("impossible to create Services for")
        return err
    }

    err = d.Ingresses.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create Ingresses")
        return err
    }

    err = d.Configmaps.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create Configmaps")
        return err
    }

    err = d.Secrets.Build()
    if err != nil{
        log.Error().Err(err).Str("stageId", d.stage.StageId).Msg("cannot create Secrets")
        return err
    }

    return nil
}

func (d DeployableKubernetesStage) Deploy(controller executor.DeploymentController) error {

    // Deploy Secrets
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy Secrets")
    err := d.Secrets.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Secrets, aborting")
        return err
    }

    // Deploy Configmaps
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy Configmaps")
    err = d.Configmaps.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Configmaps, aborting")
        return err
    }

    // Deploy Deployments
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy Deployments")
    err = d.Deployments.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Deployments, aborting")
        return err
    }
    // Deploy Services
    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy Services")
    err = d.Services.Deploy(controller)
    if err != nil {
        log.Error().Err(err).Msg("error deploying Services, aborting")
        return err
    }

    log.Debug().Str("stageId", d.stage.StageId).Msg("Deploy Ingresses")
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




