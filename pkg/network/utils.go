/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package network

// Some common function for networking

import (
	"fmt"

	"github.com/nalej/deployment-manager/pkg/common"
)

// Generate the tuple key and value for a nalej service to be represented.
// params:
//  serv service instance to be processed
// return:
//  variable name, variable value
func GetNetworkingName(serviceName string, organizationId string, appInstanceId string) string {
	value := fmt.Sprintf("%s-%s-%s", common.FormatName(serviceName), organizationId[0:10],
		appInstanceId[0:10])
	return value
}
