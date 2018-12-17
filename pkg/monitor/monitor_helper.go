/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

 // Helping structures to communicate the current status of deployments to the conductor monitor.

package monitor

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/pkg/login-helper"
    "github.com/nalej/grpc-cluster-api-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/executor"
    "google.golang.org/grpc/codes"
    grpc_status "google.golang.org/grpc/status"
)

const (
    // Maximum size of a batch of messages to be sent
    MessageBatchSize = 5
    // Time between notifications in milliseconds
    NotificationsSleep = 3000
)

type MonitorHelper struct {
    // Client
    Client grpc_cluster_api_go.ConductorClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewMonitorHelper(conn *grpc.ClientConn, loginHelper *login_helper.LoginHelper) executor.Monitor {
    client := grpc_cluster_api_go.NewConductorClient(conn)
    return &MonitorHelper{client, loginHelper}
}

func (m *MonitorHelper) UpdateFragmentStatus(organizationId string,deploymentId string, fragmentId string,
    appInstanceId string, status entities.FragmentStatus) {
    log.Debug().Str("fragmentId", fragmentId).Str("deploymentId", deploymentId).Str("organizationId",organizationId).
        Msg("send update fragment status")
    //TODO find how to populate the cluster id entry
    req := pbConductor.DeploymentFragmentUpdateRequest{
        OrganizationId: organizationId,
        DeploymentId: deploymentId,
        FragmentId: fragmentId,
        Status: entities.FragmentStatusToGRPC[status],
        ClusterId: common.CLUSTER_ID,
        AppInstanceId: appInstanceId,
    }

    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.Client.UpdateDeploymentFragmentStatus(ctx, &req)
    if err != nil {
        st := grpc_status.Convert(err).Code()
        if st == codes.Unauthenticated {
            errLogin := m.ClusterAPILoginHelper.RerunAuthentication()
            if errLogin != nil {
                log.Error().Err(errLogin).Msg("error during reauthentication")
            }
            ctx2, cancel2 := m.ClusterAPILoginHelper.GetContext()
            defer cancel2()
            _, err = m.Client.UpdateDeploymentFragmentStatus(ctx2, &req)
        } else {
            log.Error().Err(err).Msgf("error updating service status")
        }
    }

    if err != nil {
        log.Error().Err(err).Msg("error updating fragment status")
    }

}


func (m *MonitorHelper) UpdateServiceStatus(fragmentId string, organizationId string, instanceId string, serviceId string,
    status entities.NalejServiceStatus, toDeploy executor.Deployable) {
    // TODO report information if an only if a considerable bunch of updates are available
    // TODO improve performance by sending a bunch of updates at the same time
    // TODO remove the depdency with K8s deployable
    log.Debug().Msgf("send update service status with %s, %s, %v",fragmentId, serviceId, status)

    var endpoints [] string
    switch v := toDeploy.(type) {
    case *kubernetes.DeployableKubernetesStage:
        endpointsPerService := v.Ingresses.GetIngressesEndpoints()
        aux, found := endpointsPerService[serviceId]
        if found {
            endpoints = aux
        }
    default:
        log.Error().Interface("found type", v).Msg("unknown deployable type")

    }

    req := pbConductor.DeploymentServiceUpdateRequest{
        OrganizationId: organizationId,
        FragmentId: fragmentId,
        ClusterId: common.CLUSTER_ID,
        List: []*pbConductor.ServiceUpdate{
            {ApplicationInstanceId: instanceId,
            ServiceInstanceId: serviceId,
            OrganizationId: organizationId,
            Status: entities.ServiceStatusToGRPC[status],
            ClusterId: common.CLUSTER_ID,
            Endpoints: endpoints,
            },
        },
    }


    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.Client.UpdateServiceStatus(ctx, &req)
    if err != nil {
        st := grpc_status.Convert(err).Code()
        if st == codes.Unauthenticated {
            errLogin := m.ClusterAPILoginHelper.RerunAuthentication()
            if errLogin != nil {
                log.Error().Err(errLogin).Msg("error during reauthentication")
            }
            ctx2, cancel2 := m.ClusterAPILoginHelper.GetContext()
            defer cancel2()
            _, err = m.Client.UpdateServiceStatus(ctx2, &req)
        } else {
            log.Error().Err(err).Msgf("error updating service status")
        }

    }
}
