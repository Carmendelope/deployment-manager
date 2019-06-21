/*
 *  Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

 // The helping monitor informs conductor monitoring about the current status of deploying/deployed applications.

package monitor

import (
    "github.com/nalej/deployment-manager/internal/entities"
    "github.com/nalej/deployment-manager/internal/structures/monitor"
    "github.com/nalej/deployment-manager/pkg/config"
    "github.com/nalej/deployment-manager/pkg/executor"
    "github.com/nalej/deployment-manager/pkg/login-helper"
    pbApplication "github.com/nalej/grpc-application-go"
    "github.com/nalej/grpc-cluster-api-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    grpc_status "google.golang.org/grpc/status"
    "time"
)

const(
    // Time to sleep between checks
    CheckSleepTime = 5
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
            m.UpdateStatus()
        }
    }
}


func (m *MonitorHelper) sendFragmentStatus(req pbConductor.DeploymentFragmentUpdateRequest) {
    log.Debug().Str("status",req.Status.String()).Str("fragmentId", req.FragmentId).
        Str("deploymentId", req.DeploymentId).Str("organizationId",req.OrganizationId).
        Msg("send update fragment status")

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
    log.Debug().Str("status",req.Status.String()).Str("fragmentId", req.FragmentId).
        Str("deploymentId", req.DeploymentId).Str("organizationId",req.OrganizationId).
        Msg("send fragment update done")
}

func (m *MonitorHelper) UpdateStatus() {

    notificationPending := m.Monitored.GetPendingNotifications()
    if len(notificationPending) == 0 {
        // nothing to do
        return
    }

    log.Info().Msgf("monitored %d fragments with %d services and %d resources", m.Monitored.GetNumFragments(),
            m.Monitored.GetNumServices(),m.Monitored.GetNumResources())


    log.Debug().Int("pendingNotifications",len(notificationPending)).Msg("there are pending notifications")
    clusterId := config.GetConfig().ClusterId
    // notify fragments. This is equivalent to notify an entry
    for _, entry := range notificationPending {
        list := make([]*pbConductor.ServiceUpdate,0)
        for _, serv := range entry.Services {

            endpoints := make([]*pbApplication.EndpointInstance,len(serv.Endpoints))
            for i, e := range serv.Endpoints {
                endpoints[i] = e.ToGRPC()
            }

            x := &pbConductor.ServiceUpdate{
                ApplicationId: serv.AppDescriptorId,
                ApplicationInstanceId: serv.AppInstanceId,
                ServiceGroupId: serv.ServiceGroupId,
                ServiceGroupInstanceId: serv.ServiceGroupInstanceId,
                ServiceId: serv.ServiceID,
                ServiceName: serv.ServiceName,
                ServiceInstanceId:     serv.ServiceInstanceID,
                OrganizationId:        serv.OrganizationId,
                Status:                entities.ServiceStatusToGRPC[serv.Status],
                ClusterId:             clusterId,
                Endpoints:             endpoints,
                Info:                  serv.Info,
            }
            list = append(list,x)
        }

        req := pbConductor.DeploymentServiceUpdateRequest{
            OrganizationId: entry.OrganizationId,
            FragmentId:     entry.FragmentId,
            ClusterId:      clusterId,
            List:           list,
            }
        m.sendUpdateService(req)

        // Set a request for the fragment
        reqApp := pbConductor.DeploymentFragmentUpdateRequest{
            OrganizationId: entry.OrganizationId,
            DeploymentId:   entry.DeploymentId,
            FragmentId:     entry.FragmentId,
            Status:         entities.FragmentStatusToGRPC[entry.Status],
            ClusterId:      clusterId,
            AppInstanceId:  entry.AppInstanceId,
            Info:           entry.Info,
        }
        m.sendFragmentStatus(reqApp)
    }
    m.Monitored.ResetServicesUnnotifiedStatus()
}


func (m *MonitorHelper) sendUpdateService(req pbConductor.DeploymentServiceUpdateRequest) {
    log.Debug().Str("fragmentId", req.FragmentId).
        Str("organizationId",req.OrganizationId).
        Msg("send update service status")
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
    log.Debug().Str("fragmentId", req.FragmentId).
        Str("organizationId",req.OrganizationId).
        Msg("send update service status done")
}

