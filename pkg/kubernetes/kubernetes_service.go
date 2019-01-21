/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/utils"
    pbConductor "github.com/nalej/grpc-conductor-go"
    apiv1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    v12 "k8s.io/client-go/kubernetes/typed/core/v1"
    "k8s.io/client-go/kubernetes"
    "github.com/rs/zerolog/log"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/executor"
    "fmt"
)

// Deployable Services
//--------------------

type DeployableServices struct {
    // kubernetes Client
    client v12.ServiceInterface
    // stage associated with these resources
    stage *pbConductor.DeploymentStage
    // instance id
    instanceID string
    // namespace name descriptor
    targetNamespace string
    // serviceId -> serviceInstance
    services map[string]apiv1.Service
    // service_id -> zt-agent service
    ztAgents map[string]apiv1.Service
}

func NewDeployableService(client *kubernetes.Clientset, stage *pbConductor.DeploymentStage,
    instanceID string, targetNamespace string) *DeployableServices {

    return &DeployableServices{
        client: client.CoreV1().Services(targetNamespace),
        stage: stage,
        instanceID: instanceID,
        targetNamespace: targetNamespace,
        services: make(map[string]apiv1.Service,0),
        ztAgents: make(map[string]apiv1.Service,0),
    }
}

func(d *DeployableServices) GetId() string {
    return d.stage.StageId
}

func(s *DeployableServices) Build() error {
    // TODO check potential errors
    services := make(map[string]apiv1.Service,0)
    ztServices := make(map[string]apiv1.Service,0)
    for serviceIndex, service := range s.stage.Services {
        log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(s.stage.Services))
        extendedLabels := service.Labels
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
        extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = s.stage.StageId
        extendedLabels[utils.NALEJ_ANNOTATION_INSTANCE_ID] = s.instanceID
        ports := getServicePorts(service.ExposedPorts)
        if ports!=nil{
            k8sService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.targetNamespace,
                    Name: common.FormatName(service.Name),
                    Labels: extendedLabels,
                },
                Spec: apiv1.ServiceSpec{
                    ExternalName: common.FormatName(service.Name),
                    Ports: ports,
                    // TODO remove by default we use clusterip.
                    Type: apiv1.ServiceTypeNodePort,
                    Selector: service.Labels,
                },
            }
            services[service.ServiceId] = k8sService

            // Create the zt-agent service
            // Set a different set of labels to identify this agent
            ztAgentLabels := map[string]string {
                "agent": "zt-agent",
                "app": service.Labels["app"],
                utils.NALEJ_ANNOTATION_SERVICE_ID:  service.ServiceId,
                utils.NALEJ_ANNOTATION_STAGE_ID: s.stage.StageId,
                utils.NALEJ_ANNOTATION_INSTANCE_ID: s.instanceID,
            }

            ztServiceName := fmt.Sprintf("zt-%s",common.FormatName(service.Name))
            ztService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.targetNamespace,
                    Name: ztServiceName,
                    Labels: ztAgentLabels,
                },
                Spec: apiv1.ServiceSpec{
                    ExternalName: ztServiceName,
                    Ports: getServicePorts(service.ExposedPorts),
                    // TODO remove by default we use clusterip.
                    Type: apiv1.ServiceTypeNodePort,
                    Selector: ztAgentLabels,
                },
            }
            ztServices[service.ServiceId] = ztService

            log.Debug().Interface("deployment", k8sService).Msg("generated deployment")

        } else {
            log.Debug().Msgf("No k8s service is generated for %s",service.ServiceId)
        }
    }

    // add the created Services
    s.services = services
    s.ztAgents = ztServices
    return nil
}

func(s *DeployableServices) Deploy(controller executor.DeploymentController) error {
    for serviceId, serv := range s.services {
        created, err := s.client.Create(&serv)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service %s",serv.Name)
            return err
        }
        log.Debug().Str("uid",string(created.GetUID())).Str("appInstanceID",s.instanceID).
            Str("serviceID", serviceId).Msg("add service resource to be monitored")
        res := entities.NewMonitoredPlatformResource(string(created.GetUID()), s.instanceID, serviceId,"")
        controller.AddMonitoredResource(&res)
    }

    // Create Services for agents
    for serviceId, serv := range s.ztAgents {
        created, err := s.client.Create(&serv)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service agent %s",serv.Name)
            return err
        }
        log.Debug().Str("uid",string(created.GetUID())).Str("appInstanceID",s.instanceID).
            Str("serviceID", serviceId).Msg("add zt-agent service resource to be monitored")
        res := entities.NewMonitoredPlatformResource(string(created.GetUID()), s.instanceID, serviceId,"")
        controller.AddMonitoredResource(&res)
    }
    return nil
}

func(s *DeployableServices) Undeploy() error {
    for _, serv := range s.services {
        err := s.client.Delete(common.FormatName(serv.Name), metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error deleting service %s",serv.Name)
            return err
        }
    }
    // undeploy zt agents
    for _, serv := range s.ztAgents {
        err := s.client.Delete(serv.Name, metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error deleting service agent %s",serv.Name)
            return err
        }
    }
    return nil
}