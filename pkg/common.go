/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package pkg

import (
	"fmt"
	"strings"
)

const(
	// Maximum namespace length
	NamespaceLength = 63
)

// Set of common functions for all the structures.


// IP for the manager in charge of this cluster. This is required to set local environments when running
// local instances.
var MANAGER_CLUSTER_IP string

// Port of the manager cluster service.
var MANAGER_CLUSTER_PORT string

// Deployment manager address
var DEPLOYMENT_MANAGER_ADDR string

// Cluster ID
var CLUSTER_ID string

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

// Format a string to
func FormatName(name string) string {
	aux := strings.ToLower(name)
	// replace any space
	aux = strings.Replace(aux, " ", "", -1)
	return aux
}