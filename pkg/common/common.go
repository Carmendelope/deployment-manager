/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package common

import (
	"fmt"
	"strings"
)

const(
	// Maximum namespace length
	NamespaceLength = 63
	// buzzword to check all services
	AllServices = "all"
)


// Set of common functions for all the structures.

// TODO This variables must not be global, move them to the configuration.

// IP for the manager in charge of this cluster. This is required to set local environments when running
// local instances.
// Deprecated: Use config.ClusterAPIHostname
var MANAGER_CLUSTER_IP string

// Port of the manager cluster service.
// Deprecated: Use config.ClusterAPIPort
var MANAGER_CLUSTER_PORT string

// Deployment manager address
// Deprecated: Use config.DeploymentMgrAddress
var DEPLOYMENT_MANAGER_ADDR string

// Cluster ID
// TODO Create a new variable in the configuration.
var CLUSTER_ID string

// ClusterEnvironemt such as aws/google/azure/nalejCustom ....
var CLUSTER_ENV string

// Return the namespace associated with a service.
//  params:
//   organizationId
//   appInstanceId
//  return:
//   associated namespace
func GetNamespace(organizationId string, appInstanceId string) string {
	target := fmt.Sprintf("%s-%s", organizationId, appInstanceId)
	// check if the namespace is larger than the allowed k8s namespace length
	if len(target) > NamespaceLength {
		return target[:NamespaceLength]
	}
	return target
}

// Format a string removing white spaces and going lowercase
func FormatName(name string) string {
	aux := strings.ToLower(name)
	// replace any space
	aux = strings.Replace(aux, " ", "", -1)
	return aux
}