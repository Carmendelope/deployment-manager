/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package common

import (
	"fmt"
	"strings"
	"regexp"
)

const(
	// Maximum length for a name in kubernetes
	MaxNameLength = 63
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

// Return the namespace associated with a service.
//  params:
//   organizationId
//   appInstanceId
//   numRetry
//  return:
//   associated namespace
func GetNamespace(organizationId string, appInstanceId string, numRetry int) string {
	target := fmt.Sprintf("%d-%s-%s", numRetry,organizationId[0:18], appInstanceId)
	// check if the namespace is larger than the allowed k8s namespace length
	if len(target) > NamespaceLength {
		return target[len(target)-NamespaceLength:]
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

// GeneratePVCName creates a name for a PVC using the serviceGroupId, serviceId and the index.
func GeneratePVCName(groupId string, serviceId string, index string) string {
	fullName := fmt.Sprintf("%s%s-%s", groupId, serviceId, index)
	if len(fullName) > MaxNameLength {
		return fullName[len(fullName)-MaxNameLength:]
	}
	return fullName
}

// Deprecated: use GeneratePVCName
func GetNamePVC(name string, id string, index string) string {
    // remove special chars and spaces except -.
    //https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
    // 253 chars, lower case alphanumeric and "-." only

    reg,_ := regexp.Compile("[^-.a-zA-Z0-9]+")   // except these chars replace everything with ""
    name = reg.ReplaceAllString(strings.ToLower(name),"")
    id = reg.ReplaceAllString(strings.ToLower(id),"")
    // Lets try to restrict name and id to NamespaceLength, which should be enough.
    if len(id) > NamespaceLength-2 {
        id = id[:NamespaceLength-2]
    }
    // Ignore name- PVC name is limited to 63 char.
    if len(name) > NamespaceLength-2 {
        name = name[:NamespaceLength-2]
    }
    return fmt.Sprintf("%s-%s",id,index)
}