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
)

type MonitorHelper struct {
    client pbConductor.ConductorMonitorClient
}

func NewMonitorHelper(conn *grpc.ClientConn) *MonitorHelper {
    client := pbConductor.NewConductorMonitorClient(conn)
    return &MonitorHelper{client}
}

func (m *MonitorHelper) UpdateFragmentStatus(fragmentId string, status entities.FragmentStatus) {
    //TODO find how to populate the cluster id entry
    req := pbConductor.DeploymentFragmentUpdateRequest{
        FragmentId: fragmentId,
        Status: entities.FragmentStatusToGRPC[status],
        ClusterId: "fill this with the cluster id",
    }
    _, err := m.client.UpdateDeploymentFragmentStatus(context.Background(), &req)
    if err != nil {
        log.Error().Err(err).Msgf("error updating fragment status")
    }
}


func (m *MonitorHelper) UpdateServiceStatus(fragmentId string, serviceId string, status entities.ServiceStatus) {
    // TODO report information if an only if a considerable bunch of updates are available

    req := pbConductor.DeploymentServiceUpdateRequest{
        FragmentId: fragmentId,
        ClusterId: "fill this with cluster id",
        List: []*pbConductor.ServiceUpdate{
            {AppInstanceId: serviceId, Status: entities.ServiceStatusToGRPC[status]}},
    }
    _, err := m.client.UpdateServiceStatus(context.Background(), &req)
    if err != nil {
        log.Error().Err(err).Msgf("error updating service status")
    }
}
