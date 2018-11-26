/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
    pbNetwork "github.com/nalej/grpc-network-go"
    "context"
    "fmt"
    "google.golang.org/grpc"
)

type Manager struct{
    // Networking manager client
    NetClient pbNetwork.NetworksClient
    // DNS manager client
    DNSClient pbNetwork.DNSClient
}

func NewManager(connection *grpc.ClientConn) *Manager{
    // Network client
    netClient := pbNetwork.NewNetworksClient(connection)
    // DNS client
    dnsClient := pbNetwork.NewDNSClient(connection)

    return &Manager{netClient, dnsClient}
}

func (m *Manager) AuthorizeNetworkMembership(organizationId string, networkId string, memberId string) error {
    req := pbNetwork.AuthorizeMemberRequest{
        OrganizationId: organizationId,
        NetworkId: networkId,
        MemberId: memberId,
    }
    _, err := m.NetClient.AuthorizeMember(context.Background(), &req)

    return err

}

func (m *Manager) RegisterNetworkEntry(organizationId string, networkId string, serviceName string, ip string) error {

    // Create the FQDN for this service
    fqdn := fmt.Sprintf("%s.%s",serviceName,organizationId)

    req := pbNetwork.AddDNSEntryRequest{
        NetworkId: networkId,
        OrganizationId: organizationId,
        Ip: ip,
        Fqdn: fqdn,
    }
    _, err := m.DNSClient.AddDNSEntry(context.Background(), &req)

    return err
}