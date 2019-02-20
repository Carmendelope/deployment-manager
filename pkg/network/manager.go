/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
    "github.com/nalej/deployment-manager/pkg/login-helper"
    "github.com/nalej/derrors"
    "github.com/nalej/grpc-cluster-api-go"
    pbNetwork "github.com/nalej/grpc-network-go"
    "google.golang.org/grpc"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc/codes"
    grpc_status "google.golang.org/grpc/status"
)

type Manager struct{
    // ClusterAPI to send information back related to the network manager.
    ClusterAPIClient grpc_cluster_api_go.NetworkManagerClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewManager(connection *grpc.ClientConn, helper *login_helper.LoginHelper) *Manager{
    // Network & DNS client
    clusterAPIClient := grpc_cluster_api_go.NewNetworkManagerClient(connection)
    return &Manager{
        ClusterAPIClient: clusterAPIClient,
        ClusterAPILoginHelper: helper,
        }
}

func (m *Manager) AuthorizeNetworkMembership(organizationId string, networkId string, memberId string) derrors.Error {
    req := pbNetwork.AuthorizeMemberRequest{
        OrganizationId: organizationId,
        NetworkId: networkId,
        MemberId: memberId,
    }

    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()
    _, errAuth := m.ClusterAPIClient.AuthorizeMember(ctx, &req)

    if errAuth != nil {
        st := grpc_status.Convert(errAuth).Code()
        if st == codes.Unauthenticated {
            errLogin := m.ClusterAPILoginHelper.RerunAuthentication()
            if errLogin != nil {
                log.Error().Err(errLogin).Msg("error during reauthentication")
            }
            ctx2, cancel2 := m.ClusterAPILoginHelper.GetContext()
            defer cancel2()
            _, errAuth = m.ClusterAPIClient.AuthorizeMember(ctx2, &req)
        } else {
            log.Error().Err(errAuth).Msgf("error updating service status")
        }
    }

    if errAuth != nil {
        return derrors.NewGenericError(errAuth.Error())
    }

    return nil

}

func (m *Manager) RegisterNetworkEntry(organizationId string, organizationName string, appInstanceId string,
    networkId string, serviceName string, ip string, serviceGroupInstanceId string, serviceAppInstanceId string) derrors.Error {

    // Create the FQDN for this service
    fqdn := GetNetworkingName(serviceName, organizationId, serviceGroupInstanceId, serviceAppInstanceId)


    req := pbNetwork.AddDNSEntryRequest{
        NetworkId: networkId,
        OrganizationId: organizationId,
        OrganizationName: organizationName,
        Ip: ip,
        Fqdn: fqdn,
        AppInstanceId: appInstanceId,
        ServiceName: serviceName,
    }
    ctx, cancel := m.ClusterAPILoginHelper.GetContext()
    defer cancel()

    _, err := m.ClusterAPIClient.AddDNSEntry(ctx, &req)

    if err != nil {
        st := grpc_status.Convert(err).Code()
        if st == codes.Unauthenticated {
            errLogin := m.ClusterAPILoginHelper.RerunAuthentication()
            if errLogin != nil {
                log.Error().Err(errLogin).Msg("error during reauthentication")
            }
            ctx2, cancel2 := m.ClusterAPILoginHelper.GetContext()
            defer cancel2()
            _, err = m.ClusterAPIClient.AddDNSEntry(ctx2, &req)
        } else {
            log.Error().Err(err).Msgf("error updating service status")
        }
    }

    if err != nil {
        return derrors.NewGenericError(err.Error())
    }

    return nil
}
