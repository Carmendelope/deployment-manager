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

package utils

// Collection of variable names to be used in the project

const (

	// Environment variable declared in the cluster with the cluster id
	NALEJ_ANNOTATION_CLUSTER_ID = "cluster-id"
	// Identifier for the ZT network
	NALEJ_ANNOTATION_ZT_NETWORK_ID = "nalej-zt-network-id"
	// Boolean value setting a container to be used as an inbound proxy
	NALEJ_ANNOTATION_IS_PROXY = "nalej-is-proxy"
	// Address of the manager cluster
	NALEJ_ANNOTATION_MANAGER_ADDR = "nalej-manager-addr"
	// Annotation for the deployment id
	NALEJ_ANNOTATION_DEPLOYMENT_ID = "nalej-deployment-id"
	// Annotation for the deployment fragment
	NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT = "nalej-deployment-fragment"
	// Annotation for the organization
	NALEJ_ANNOTATION_ORGANIZATION_ID = "nalej-organization"
	// Annotation for the organization name
	NALEJ_ANNOTATION_ORGANIZATION_NAME = "nalej-organization-name"
	// Annotation application descriptor
	NALEJ_ANNOTATION_APP_DESCRIPTOR = "nalej-app-descriptor"
	// Annotation application descriptor
	NALEJ_ANNOTATION_APP_DESCRIPTOR_NAME = "nalej-app-descriptor-name"
	// Annotation for the application name
	NALEJ_ANNOTATION_APP_NAME = "nalej-app-name"
	// Annotation application instance
	NALEJ_ANNOTATION_APP_INSTANCE_ID = "nalej-app-instance-id"
	// Annotation for metadata to identify the stage for these deployments
	NALEJ_ANNOTATION_STAGE_ID = "nalej-stage-id"
	// Service name
	NALEJ_ANNOTATION_SERVICE_NAME = "nalej-service-name"
	// FQDN for a service
	NALEJ_ANNOTATION_SERVICE_FQDN = "nalej-service-fqdn"
	// Annotation for metadata to identify the service for these deployments
	NALEJ_ANNOTATION_SERVICE_ID = "nalej-service-id"
	// Annotation for metadata to identify the service instance for these deployments
	NALEJ_ANNOTATION_SERVICE_INSTANCE_ID = "nalej-service-instance-id"
	// Annotation for metadata to identify the group service
	NALEJ_ANNOTATION_SERVICE_GROUP_ID = "nalej-service-group-id"
	// Annotation for metadata with the service group name
	NALEJ_ANNOTATION_SERVICE_GROUP_NAME = "nalej-service-group-name"
	// Annotation for metadata to identify the group service
	NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID = "nalej-service-group-instance-id"

	// Annotation for Storage Type
	NALEJ_ANNOTATION_STORAGE_TYPE = "nalej-storage-type"

	// Annotation for metadata to identify endpoints for ingress
	NALEJ_ANNOTATION_INGRESS_ENDPOINT = "nalej-endpoint"
	// Annotation for metadata to identify the security rule.
	NALEJ_ANNOTATION_SECURITY_RULE_ID = "nalej-security-rule-id"

	// TODO review this notation. It must be uppercase
	NALEJ_ANNOTATION_SERVICE_PURPOSE             = "nalej-service-purpose"
	NALEJ_ANNOTATION_VALUE_DEVICE_GROUP_SERVICE  = "device-group"
	NALEJ_ANNOTATION_VALUE_LOAD_BALANCER_SERVICE = "load-balancer"

	// NALEJ_ANNOTATION_DG_SECRETS contains the name of the environemnt variable to store device group secrets.
	NALEJ_ANNOTATION_DG_SECRETS = "NALEJ_DG_SHARED_SECRETS"

	// This is the definition of variables.
	// Environment variable declared in the cluster with the cluster id
	NALEJ_ENV_CLUSTER_ID = "CLUSTER_ID"
	// Boolean value setting a container to be used as an inbound proxy
	NALEJ_ENV_IS_PROXY = "NALEJ_IS_PROXY"
	// Address of the manager cluster
	NALEJ_ENV_MANAGER_ADDR = "NALEJ_MANAGER_ADDR"
	// Annotation for the deployment id
	NALEJ_ENV_DEPLOYMENT_ID = "NALEJ_DEPLOYMENT_ID"
	// Annotation for the deployment fragment
	NALEJ_ENV_DEPLOYMENT_FRAGMENT = "NALEJ_DEPLOYMENT_FRAGMENT"
	// Annotation for the organization
	NALEJ_ENV_ORGANIZATION_ID = "NALEJ_ORGANIZATION"
	// Annotation for the organization name
	NALEJ_ENV_ORGANIZATION_NAME = "NALEJ_ORGANIZATION_NAME"
	// Annotation application descriptor
	NALEJ_ENV_APP_DESCRIPTOR = "NALEJ_APP_DESCRIPTOR"
	// Annotation for the application name
	NALEJ_ENV_APP_NAME = "NALEJ_APP_NAME"
	// Annotation application instance
	NALEJ_ENV_APP_INSTANCE_ID = "NALEJ_APP_INSTANCE_ID"
	// Annotation for metadata to identify the stage for these deployments
	NALEJ_ENV_STAGE_ID = "NALEJ_STAGE_ID"
	// Service name
	NALEJ_ENV_SERVICE_NAME = "NALEJ_SERVICE_NAME"
	// FQDN for a service
	NALEJ_ENV_SERVICE_FQDN = "NALEJ_SERVICE_FQDN"
	// Annotation for metadata to identify the service for these deployments
	NALEJ_ENV_SERVICE_ID = "NALEJ_SERVICE_ID"
	// Annotation for metadata to identify the service instance for these deployments
	NALEJ_ENV_SERVICE_INSTANCE_ID = "NALEJ_SERVICE_INSTANCE_ID"
	// Annotation for metadata to identify the group service
	NALEJ_ENV_SERVICE_GROUP_ID = "NALEJ_SERVICE_GROUP_ID"
	// Annotation for metadata to identify the group service
	NALEJ_ENV_SERVICE_GROUP_INSTANCE_ID = "NALEJ_SERVICE_GROUP_INSTANCE_ID"
)
