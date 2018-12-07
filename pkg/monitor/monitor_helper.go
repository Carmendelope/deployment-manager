/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

 // Helping structures to communicate the current status of deployments to the conductor monitor.

package monitor

import (
    "github.com/nalej/deployment-manager/internal/entities"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbClusterAPI "github.com/nalej/grpc-cluster-api-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/pkg"
    "github.com/nalej/deployment-manager/pkg/login-helper"
)

const (
    // Maximum size of a batch of messages to be sent
    MessageBatchSize = 5
    // Time between notifications in milliseconds
    NotificationsSleep = 3000
    // Services wildcard is a reserved word to indicate all the services in a deployment stage
    AllServices = "all"
)

type MonitorHelper struct {
    // Client
    Client pbClusterAPI.ConductorClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewMonitorHelper(conn *grpc.ClientConn, loginHelper *login_helper.LoginHelper) *MonitorHelper {
    client := pbClusterAPI.NewConductorClient(conn)
    return &MonitorHelper{client, loginHelper}
}

func (m *MonitorHelper) UpdateFragmentStatus(organizationId string,deploymentId string, fragmentId string,
    appInstanceId string, status entities.FragmentStatus) {
    //TODO find how to populate the cluster id entry
    req := pbConductor.DeploymentFragmentUpdateRequest{
        OrganizationId: organizationId,
        DeploymentId: deploymentId,
        FragmentId: fragmentId,
        Status: entities.FragmentStatusToGRPC[status],
        ClusterId: pkg.CLUSTER_ID,
        AppInstanceId: appInstanceId,
    }

    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()
    _, err := m.Client.UpdateDeploymentFragmentStatus(ctx, &req)
    if err != nil {
        log.Error().Err(err).Msgf("error updating fragment status")
    }
}


func (m *MonitorHelper) UpdateServiceStatus(fragmentId string, organizationId string, instanceId string, serviceId string,
    status entities.NalejServiceStatus) {
    // TODO report information if an only if a considerable bunch of updates are available
    // TODO improve performance by sending a bunch of updates at the same time
    log.Debug().Msgf("send update service status with %s, %s, %v",fragmentId, serviceId, status)
    req := pbConductor.DeploymentServiceUpdateRequest{
        OrganizationId: organizationId,
        FragmentId: fragmentId,
        ClusterId: "fill this with cluster id",
        List: []*pbConductor.ServiceUpdate{
            {ApplicationInstanceId: instanceId,
            ServiceInstanceId: serviceId,
            OrganizationId: organizationId,
            Status: entities.ServiceStatusToGRPC[status]},
            },
    }
    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()
    _, err := m.Client.UpdateServiceStatus(ctx, &req)
    if err != nil {
        log.Error().Err(err).Msgf("error updating service status")
    }
}
