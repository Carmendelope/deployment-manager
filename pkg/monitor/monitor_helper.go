/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

 // Helping structures to communicate the current status of deployments to the conductor monitor.

package monitor

import (
    pbConductor "github.com/nalej/grpc-conductor-go"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/internal/entities"
    "context"
    "github.com/rs/zerolog/log"
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
    client pbConductor.ConductorMonitorClient
}

func NewMonitorHelper(conn *grpc.ClientConn) *MonitorHelper {
    client := pbConductor.NewConductorMonitorClient(conn)
    return &MonitorHelper{client}
}

func (m *MonitorHelper) UpdateFragmentStatus(organizationId string,deploymentId string, fragmentId string,
    appInstanceId string, status entities.FragmentStatus) {
    //TODO find how to populate the cluster id entry
    req := pbConductor.DeploymentFragmentUpdateRequest{
        OrganizationId: organizationId,
        DeploymentId: deploymentId,
        FragmentId: fragmentId,
        Status: entities.FragmentStatusToGRPC[status],
        ClusterId: "fill this with the cluster id",
        AppInstanceId: appInstanceId,
    }
    _, err := m.client.UpdateDeploymentFragmentStatus(context.Background(), &req)
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
    _, err := m.client.UpdateServiceStatus(context.Background(), &req)
    if err != nil {
        log.Error().Err(err).Msgf("error updating service status")
    }
}
