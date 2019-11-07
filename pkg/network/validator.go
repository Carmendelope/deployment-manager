/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
