/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

 // The helping monitor informs conductor monitoring about the current status of deploying/deployed applications.

package monitor

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/login-helper"
    "github.com/nalej/grpc-cluster-api-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/pkg/common"
    "github.com/nalej/deployment-manager/pkg/executor"
    "google.golang.org/grpc/codes"
    grpc_status "google.golang.org/grpc/status"
    "time"
)

const(
    // Time to sleep between checks
    CheckSleepTime = 15
)

type MonitorHelper struct {
    // Client
    Client grpc_cluster_api_go.ConductorClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
    // Structure containing monitored entries
    Monitored monitor.MonitoredInstances
}

func NewMonitorHelper(conn *grpc.ClientConn, loginHelper *login_helper.LoginHelper,
    monitored monitor.MonitoredInstances) executor.Monitor {
    client := grpc_cluster_api_go.NewConductorClient(conn)
    return &MonitorHelper{Client: client, ClusterAPILoginHelper: loginHelper, Monitored: monitored}
}

// This function periodically informs conductor about the status of deployed and on deployment services.
func (m *MonitorHelper) Run() {
    log.Info().Msg("Start monitor helper...")
    tick := time.Tick(time.Second * CheckSleepTime)
    for {
        select {
        case <-tick:
            // TODO Send the status
            m.UpdateStatus()
        }
    }
}


func (m *MonitorHelper) sendFragmentStatus(req pbConductor.DeploymentFragmentUpdateRequest) {
    log.Debug().Str("fragmentId", req.FragmentId).Str("deploymentId", req.DeploymentId).
        Str("organizationId",req.OrganizationId).Msg("send update fragment status")
    //TODO find how to populate the cluster id entry

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

func (m *MonitorHelper) UpdateStatus() {
    notificationPending := m.Monitored.GetPendingNotifications()
    if len(notificationPending) == 0 {
        // nothing to do
        return
    }
    // notify fragments. This is equivalent to notify an app
    for _, app := range notificationPending {
        list := make([]*pbConductor.ServiceUpdate,0)
        for _, serv := range app.Services {
            x := &pbConductor.ServiceUpdate{
                ApplicationInstanceId: serv.InstanceId,
                ServiceInstanceId:     serv.ServiceID,
                OrganizationId:        serv.OrganizationId,
                Status:                entities.ServiceStatusToGRPC[serv.Status],
                ClusterId:             common.CLUSTER_ID,
                Endpoints:             serv.Endpoints,
                Info:                  serv.Info,
            }
            list = append(list,x)
        }

        req := pbConductor.DeploymentServiceUpdateRequest{
            OrganizationId: app.OrganizationId,
            FragmentId:     app.FragmentId,
            ClusterId:      common.CLUSTER_ID,
            List:           list,
            }
        m.sendUpdateService(req)

        // Set a request for the frament
        reqApp := pbConductor.DeploymentFragmentUpdateRequest{
            OrganizationId: app.OrganizationId,
            DeploymentId: app.DeploymentId,
            FragmentId: app.FragmentId,
            Status: entities.FragmentStatusToGRPC[app.Status],
            ClusterId: common.CLUSTER_ID,
            AppInstanceId: app.InstanceId,
            Info: app.Info,
        }
        m.sendFragmentStatus(reqApp)
    }
    m.Monitored.ResetServicesUnnotifiedStatus()
}


func (m *MonitorHelper) sendUpdateService(req pbConductor.DeploymentServiceUpdateRequest) {
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

