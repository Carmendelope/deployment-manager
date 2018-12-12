/*
 * Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package network

// Some common function for networking

import(
    "fmt"
    "github.com/nalej/deployment-manager/pkg"
)

// Return the networking name for a given service.
func GetNetworkingName(serviceName string, organizationName string, appInstanceId string) string {
    name := fmt.Sprintf("%s-%s-%s.service.nalej", pkg.FormatName(serviceName), pkg.FormatName(organizationName), appInstanceId[0:5])
    return name
}
