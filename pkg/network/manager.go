/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
    "context"
    "fmt"
    pbNetwork "github.com/nalej/grpc-network-go"
    pbClientAPI "github.com/nalej/grpc-cluster-api-go"
    "google.golang.org/grpc"
)

type Manager struct{
    // Networking & DNS manager client
    ClusterAPIClient pbClientAPI.NetworkManagerClient
}

func NewManager(connection *grpc.ClientConn) *Manager{
    // Network & DNS client
    clusterAPIClient := pbClientAPI.NewNetworkManagerClient(connection)

    return &Manager{clusterAPIClient}
}

func (m *Manager) AuthorizeNetworkMembership(organizationId string, networkId string, memberId string) error {
    req := pbNetwork.AuthorizeMemberRequest{
        OrganizationId: organizationId,
        NetworkId: networkId,
        MemberId: memberId,
    }
    _, err := m.ClusterAPIClient.AuthorizeMember(context.Background(), &req)

    return err

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