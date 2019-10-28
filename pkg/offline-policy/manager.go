/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 *
 */

package offline_policy

import (
	"context"
	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/login-helper"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-common-go"
	"github.com/nalej/grpc-deployment-manager-go"
	"github.com/nalej/grpc-infrastructure-go"
	"github.com/nalej/grpc-organization-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"time"
)

const (
	DefaultTimeout =  2*time.Minute
)

type Manager struct {
	// Config
	conf config.Config
	// DM Client
	DeploymentManagerClient grpc_deployment_manager_go.DeploymentManagerClient
	// Clusters Client
	ClustersClient grpc_infrastructure_go.ClustersClient
	// Applications Client
	ApplicationsClient grpc_application_go.ApplicationsClient
	// Organizations Client
	OrganizationsClient grpc_organization_go.OrganizationsClient
	// LoginHelper Helper
	ClusterAPILoginHelper *login_helper.LoginHelper
}

func NewManager(conn *grpc.ClientConn, clusterApiLoginHelper *login_helper.LoginHelper) *Manager {
	dmClient := grpc_deployment_manager_go.NewDeploymentManagerClient(conn)
	appsClient := grpc_application_go.NewApplicationsClient(conn)
	cClient := grpc_infrastructure_go.NewClustersClient(conn)
	oClient := grpc_organization_go.NewOrganizationsClient(conn)

	return &Manager{
		DeploymentManagerClient:dmClient,
		ApplicationsClient:appsClient,
		ClustersClient:cClient,
		OrganizationsClient:oClient,
		ClusterAPILoginHelper:clusterApiLoginHelper,
	}
}

func (m *Manager) RemoveAll () derrors.Error {
	loginCtx, loginCancel := m.ClusterAPILoginHelper.GetContext()
	defer loginCancel()

	// Get organization list
	organizationList, err := m.OrganizationsClient.ListOrganizations(loginCtx,&grpc_common_go.Empty{})
	if err != nil {
		log.Error().Err(err).Msg("cannot retrieve organization list")
		return conversions.ToDerror(err)
	}

	for _, org := range organizationList.Organizations {
		// Get organization ID
		log.Debug().Str("organizationID", org.OrganizationId).Msg("checking organization clusters")
		organizationId := &grpc_organization_go.OrganizationId{
			OrganizationId:       org.OrganizationId,
		}
		clusterCtx, clusterCancel := context.WithTimeout(context.Background(), DefaultTimeout)
		defer clusterCancel()

		// Get cluster ID
		cluster, err := m.ClustersClient.GetCluster(clusterCtx, &grpc_infrastructure_go.ClusterId{
			OrganizationId:       organizationId.OrganizationId,
			ClusterId:            m.conf.ClusterId,
		})
		if err != nil {
			log.Error().Err(err).Str("cluster id", cluster.ClusterId).Msg("cannot retrieve cluster")
			return conversions.ToDerror(err)
		}

		// Get app instance list
		appInstCtx, appInstCancel := context.WithTimeout(context.Background(), DefaultTimeout)
		defer appInstCancel()
		appInstanceList, err := m.ApplicationsClient.ListAppInstances(appInstCtx, organizationId)
		if err != nil {
			log.Error().Err(err).Str("organization id", organizationId.OrganizationId).Msg("cannot retrieve app instance id")
			return conversions.ToDerror(err)
		}

		// Undeploy everything
		uCtx, uCancel := context.WithTimeout(context.Background(), DefaultTimeout)
		defer uCancel()

		for _, appInst := range appInstanceList.Instances {
			undeployRequest := grpc_deployment_manager_go.UndeployRequest{
				OrganizationId:       organizationId.OrganizationId,
				AppInstanceId:        appInst.AppInstanceId,
			}
			_, err := m.DeploymentManagerClient.Undeploy(uCtx, &undeployRequest)
			if err != nil {
				log.Error().Err(err).Str("app instance id", appInst.AppInstanceId).Msg("cannot undeploy app instance")
				return conversions.ToDerror(err)
			}
		}
	}

	return nil
}