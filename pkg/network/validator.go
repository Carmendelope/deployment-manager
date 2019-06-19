/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package network

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-deployment-manager-go"
)

func ValidSetRoute(req *grpc_deployment_manager_go.ServiceRoute) derrors.Error {
	if req.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id must not be empty")
	}
	if req.AppInstanceId == "" {
		return derrors.NewInvalidArgumentError("app_instance_id must not be empty")
	}
	if req.ServiceGroupId == "" {
		return derrors.NewInvalidArgumentError("service_group_id must not be empty")
	}
	if req.ServiceId == "" {
		return derrors.NewInvalidArgumentError("service_id must not be empty")
	}
	if req.Vsa == "" {
		return derrors.NewInvalidArgumentError("vsa must not be empty")
	}
	if !req.Drop {
		if req.RedirectToVpn == "" {
			return derrors.NewInvalidArgumentError("redirect_to_vpn must not be empty")
		}
	}
	return nil
}
