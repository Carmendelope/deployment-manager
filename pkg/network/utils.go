/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package network

// Some common function for networking

import(
    "fmt"
    "github.com/nalej/deployment-manager/pkg/common"
)

// Return the networking name for a given service.
//func GetNetworkingName(serviceName string, organizationName string, appInstanceId string) string {
//    name := fmt.Sprintf("%s-%s-%s", common.FormatName(serviceName), common.FormatName(organizationName), appInstanceId[0:5])
//    return name
//}

// Generate the tuple key and value for a nalej service to be represented.
// params:
//  serv service instance to be processed
// return:
//  variable name, variable value
func GetNetworkingName(serviceName string, organizationId string, serviceGroupInstanceId string, serviceAppInstanceId string) string {
    value := fmt.Sprintf("%s-%s-%s-%s", common.FormatName(serviceName), organizationId[0:5],
        serviceGroupInstanceId[0:5], serviceAppInstanceId[0:5])
    return value}