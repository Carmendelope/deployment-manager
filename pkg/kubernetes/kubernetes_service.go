/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/utils"
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

type ServiceInfo struct {
    ServiceId string
    ServiceInstanceId string
    Service apiv1.Service
}

type DeployableServices struct {
    // kubernetes Client
    client v12.ServiceInterface
    // Deployment metadata
    data entities.DeploymentMetadata
    // [[ServiceId, ServiceInstanceId, Service]...]
    services []ServiceInfo
    // [[ServiceId, ServiceInstanceId, Service]...]
    ztAgents []ServiceInfo
}

func NewDeployableService(client *kubernetes.Clientset, data entities.DeploymentMetadata) *DeployableServices {

    return &DeployableServices{
        client: client.CoreV1().Services(data.Namespace),
        data: data,
        services: make([]ServiceInfo,0),
        ztAgents: make([]ServiceInfo,0),
    }
}

func(d *DeployableServices) GetId() string {
    return d.data.Stage.StageId
}

func(s *DeployableServices) Build() error {
    // TODO check potential errors
    //services := make(map[string]apiv1.Service,0)
    //ztServices := make(map[string]apiv1.Service,0)
    for serviceIndex, service := range s.data.Stage.Services {
        log.Debug().Msgf("build service %s %d out of %d", service.ServiceId, serviceIndex+1, len(s.data.Stage.Services))
        extendedLabels := service.Labels
        extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION] = s.data.OrganizationId
        extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = s.data.AppDescriptorId
        extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = s.data.AppInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = s.data.Stage.StageId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
        extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
        ports := getServicePorts(service.ExposedPorts)
        if ports!=nil{
            k8sService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.data.Namespace,
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
            //services[service.ServiceId] = k8sService
            log.Debug().Str("serviceId",service.ServiceId).Str("serviceInstanceId",service.ServiceInstanceId).
                Interface("apiv1.Service",k8sService).Msg("generated k8s service")
            s.services = append(s.services, ServiceInfo{service.ServiceId, service.ServiceInstanceId, k8sService})

            // Create the zt-agent service
            // Set a different set of labels to identify this agent
            ztAgentLabels := map[string]string {
                "agent": "zt-agent",
                "app": service.Labels["app"],
                utils.NALEJ_ANNOTATION_ORGANIZATION : s.data.OrganizationId,
                utils.NALEJ_ANNOTATION_APP_DESCRIPTOR : s.data.AppDescriptorId,
                utils.NALEJ_ANNOTATION_APP_INSTANCE_ID : s.data.AppInstanceId,
                utils.NALEJ_ANNOTATION_STAGE_ID : s.data.Stage.StageId,
                utils.NALEJ_ANNOTATION_SERVICE_ID : service.ServiceId,
                utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID : service.ServiceInstanceId,
                utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID : service.ServiceGroupId,
                utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID : service.ServiceGroupInstanceId,
            }

            ztServiceName := fmt.Sprintf("zt-%s",common.FormatName(service.Name))
            ztService := apiv1.Service{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: s.data.Namespace,
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
            log.Debug().Str("serviceId",service.ServiceId).Str("serviceInstanceId",service.ServiceInstanceId).
                Interface("apiv1.Service",k8sService).Msg("generated zt-agent service")
            s.ztAgents = append(s.ztAgents, ServiceInfo{service.ServiceId, service.ServiceInstanceId, ztService})

            log.Debug().Interface("deployment", k8sService).Msg("generated deployment")

        } else {
            log.Debug().Msgf("No k8s service is generated for %s",service.ServiceId)
        }
    }

    // add the created Services
    //s.services = services
    //s.ztAgents = ztServices
    return nil
}

func(s *DeployableServices) Deploy(controller executor.DeploymentController) error {
    //for serviceId, serv := range s.services {
    for _, servInfo := range s.services {
        created, err := s.client.Create(&servInfo.Service)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service %s",servInfo.Service.Name)
            return err
        }
        log.Debug().Str("uid",string(created.GetUID())).Str("appInstanceID",s.data.AppInstanceId).
            Str("serviceID", servInfo.ServiceId).Msg("add service resource to be monitored")
        //res := entities.NewMonitoredPlatformResource(string(created.GetUID()), s.data, servInfo.ServiceId, servInfo.ServiceInstanceId,"")
        res := entities.NewMonitoredPlatformResource(string(created.GetUID()),
            created.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], created.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
            created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
            created.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
        controller.AddMonitoredResource(&res)
    }

    // Create Services for agents
    //for serviceId, serv := range s.ztAgents {
    for _, servInfo := range s.ztAgents {
        created, err := s.client.Create(&servInfo.Service)
        if err != nil {
            log.Error().Err(err).Msgf("error creating service agent %s",servInfo.Service.Name)
            return err
        }
        log.Debug().Str("uid",string(created.GetUID())).Str("appInstanceID",s.data.AppInstanceId).
            Str("serviceID", servInfo.ServiceId).Msg("add zt-agent service resource to be monitored")
        //res := entities.NewMonitoredPlatformResource(string(created.GetUID()), s.data, servInfo.ServiceId, servInfo.ServiceInstanceId,"")
        res := entities.NewMonitoredPlatformResource(string(created.GetUID()),
            created.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], created.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
            created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
            created.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
        controller.AddMonitoredResource(&res)
    }
    return nil
}

func(s *DeployableServices) Undeploy() error {
    for _, servInfo := range s.services {
        err := s.client.Delete(common.FormatName(servInfo.Service.Name), metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error deleting service %s", servInfo.Service.Name)
            return err
        }
    }
    // undeploy zt agents
    for _, servInfo := range s.ztAgents {
        err := s.client.Delete(servInfo.Service.Name, metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
        if err != nil {
            log.Error().Err(err).Msgf("error deleting service agent %s", servInfo.Service.Name)
            return err
        }
    }
    return nil
}