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

package common

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

const (
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
	// randon suffix to mitigate chances of collision
	randomSuffix := rand.Intn(100)
	target := fmt.Sprintf("%1d-%s-%s-%02d", numRetry, organizationId[0:18], appInstanceId, randomSuffix)
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

func FormatNameWithHyphen(name string) string {
	// replace any space
	aux := strings.Replace(name, " ", "-", -1)
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

	reg, _ := regexp.Compile("[^-.a-zA-Z0-9]+") // except these chars replace everything with ""
	name = reg.ReplaceAllString(strings.ToLower(name), "")
	id = reg.ReplaceAllString(strings.ToLower(id), "")
	// Lets try to restrict name and id to NamespaceLength, which should be enough.
	if len(id) > NamespaceLength-2 {
		id = id[:NamespaceLength-2]
	}
	// Ignore name- PVC name is limited to 63 char.
	if len(name) > NamespaceLength-2 {
		name = name[:NamespaceLength-2]
	}
	return fmt.Sprintf("%s-%s", id, index)
}

// Get the corresponding FQDN for a service
// params:
//  serviceName
//  organizationId
//  appInstanceId
// return:
//  corresponding FQDN for the service
func GetServiceFQDN(serviceName string, organizationId string, appInstanceId string) string {
	return fmt.Sprintf("%s-%s-%s", FormatName(serviceName), organizationId[0:10], appInstanceId[0:10])
}

// Helping function to extract a boolean pointer from a boolean.
// params:
//  b
// return:
//  The corresponding pointer.
func BoolPtr(b bool) *bool { return &b }


// Helping function for pointer conversion.
func Int32Ptr(i int32) *int32 { return &i }

// Helping function for pointer conversion.
func Int64Ptr(i int64) *int64 { return &i }