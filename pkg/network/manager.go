/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
    "context"
    "fmt"
    "github.com/nalej/deployment-manager/pkg/login-helper"
    "github.com/nalej/derrors"
    pbClientAPI "github.com/nalej/grpc-cluster-api-go"
    pbNetwork "github.com/nalej/grpc-network-go"
    "google.golang.org/grpc"
)

type Manager struct{
    // Networking & DNS manager client
    ClusterAPIClient pbClientAPI.NetworkManagerClient
    // LoginHelper Helper
    ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewManager(connection *grpc.ClientConn, helper *login_helper.LoginHelper) *Manager{
    // Network & DNS client
    clusterAPIClient := pbClientAPI.NewNetworkManagerClient(connection)
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

    _, errAuth := m.ClusterAPIClient.AuthorizeMember(m.ClusterAPILoginHelper.Ctx, &req)

    if errAuth != nil {
        return derrors.NewGenericError("error authorizing network membership", errAuth)
    }

    return nil

}

func (m *Manager) RegisterNetworkEntry(organizationId string, networkId string, serviceName string, ip string) error {

    // Create the FQDN for this service
    fqdn := fmt.Sprintf("%s-%s",serviceName,organizationId)

    req := pbNetwork.AddDNSEntryRequest{
        NetworkId: networkId,
        OrganizationId: organizationId,
        Ip: ip,
        Fqdn: fqdn,
    }
    _, err := m.ClusterAPIClient.AddDNSEntry(context.Background(), &req)

    return err
}