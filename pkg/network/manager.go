/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
	"fmt"
	"os"

	"github.com/nalej/deployment-manager/pkg/login-helper"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-cluster-api-go"
	grpc_deployment_manager_go "github.com/nalej/grpc-deployment-manager-go"
	pbNetwork "github.com/nalej/grpc-network-go"
	grpc_zt_nalej_go "github.com/nalej/grpc-zt-nalej-go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
)

type Manager struct {
	// ClusterAPI to send information back related to the network manager.
	ClusterAPIClient grpc_cluster_api_go.NetworkManagerClient
	// LoginHelper Helper
	ClusterAPILoginHelper *login_helper.LoginHelper
	// NetUpdater with the network updater for deployed pods.
	NetUpdater NetworkUpdater
}

func NewManager(connection *grpc.ClientConn, helper *login_helper.LoginHelper, K8sClient *kubernetes.Clientset) *Manager {
	// Network & DNS client
	clusterAPIClient := grpc_cluster_api_go.NewNetworkManagerClient(connection)
	netUpdater := NewKubernetesNetworkUpdater(K8sClient)
	return &Manager{
		ClusterAPIClient:      clusterAPIClient,
		ClusterAPILoginHelper: helper,
		NetUpdater:            netUpdater,
	}
}

func (m *Manager) AuthorizeNetworkMembership(organizationId string, appInstanceId string, serviceGroupInstanceId string,
	serviceInstanceId string, networkId string, memberId string, isProxy bool) derrors.Error {
	req := pbNetwork.AuthorizeMemberRequest{
		OrganizationId:               organizationId,
		NetworkId:                    networkId,
		MemberId:                     memberId,
		ServiceGroupInstanceId:       serviceGroupInstanceId,
		ServiceApplicationInstanceId: serviceInstanceId,
		AppInstanceId:                appInstanceId,
		IsProxy:                      isProxy,
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

func (m *Manager) RegisterNetworkEntry(organizationId string, appInstanceId string,
	networkId string, serviceName string, ip string, serviceGroupInstanceId string, serviceAppInstanceId string) derrors.Error {

	// Create the FQDN for this service
	fqdn := GetNetworkingName(serviceName, organizationId, appInstanceId)

	req := pbNetwork.AddDNSEntryRequest{
		OrganizationId: organizationId,
		ServiceName:    serviceName,
		Ip:             ip,
		Fqdn:           fqdn,
		Tags: []string{
			fmt.Sprintf("organizationId:%s", organizationId),
			fmt.Sprintf("appInstanceId:%s", appInstanceId),
			fmt.Sprintf("serviceGroupInstanceId:%s", serviceGroupInstanceId),
			fmt.Sprintf("serviceAppInstanceId:%s", serviceAppInstanceId),
			fmt.Sprintf("clusterId:%s", os.Getenv(utils.NALEJ_ANNOTATION_CLUSTER_ID)),
			fmt.Sprintf("networkId:%s", networkId),
		},
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

// SetServiceRoute setups an iptables DNAT for a given service
func (m *Manager) SetServiceRoute(request *grpc_deployment_manager_go.ServiceRoute) derrors.Error {
	// Get target namespace
	targetNS, exist, err := m.NetUpdater.GetTargetNamespace(request.OrganizationId, request.AppInstanceId)
	if err != nil {
		return err
	}
	if !exist {
		return derrors.NewNotFoundError("no namespace found for given organization ID and app instance ID")
	}
	// Get the list of pods/k8s services to be updated
	pods, err := m.NetUpdater.GetPodsForApp(targetNS, request.OrganizationId, request.AppInstanceId)
	if err != nil {
		return err
	}
	// Update routes
	route := &grpc_zt_nalej_go.Route{
		Vsa:           request.Vsa,
		RedirectToVpn: request.RedirectToVpn,
		Drop:          request.Drop,
	}
	err = m.NetUpdater.UpdatePodsRoute(pods, route)
	if err != nil {
		return err
	}
	return nil
}
