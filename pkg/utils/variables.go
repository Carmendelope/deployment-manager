/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package utils

// Collection of variable names to be used in the project

const (

    // Environment variable declared in the cluster with the cluster id
    NALEJ_CLUSTER_ID = "CLUSTER_ID"
    // Identifier for the ZT network
    NALEJ_ANNOTATION_ZT_NETWORK_ID = "NALEJ_ZT_NETWORK_ID"
    // Boolean value setting a container to be used as an inbound proxy
    NALEJ_ANNOTATION_IS_PROXY = "NALEJ_IS_PROXY"
    // Address of the manager cluster
    NALEJ_ANNOTATION_MANAGER_ADDR = "NALEJ_MANAGER_ADDR"
    // Annotation for the deployment id
    NALEJ_ANNOTATION_DEPLOYMENT_ID = "NALEJ_DEPLOYMENT_ID"
    // Annotation for the deployment fragment
    NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT = "NALEJ_DEPLOYMENT_FRAGMENT"
    // Annotation for the organization
    NALEJ_ANNOTATION_ORGANIZATION_ID = "NALEJ_ORGANIZATION"
    // Annotation for the organization name
    NALEJ_ANNOTATION_ORGANIZATION_NAME = "NALEJ_ORGANIZATION_NAME"
    // Annotation application descriptor
    NALEJ_ANNOTATION_APP_DESCRIPTOR = "NALEJ_APP_DESCRIPTOR"
    // Annotation for the application name
    NALEJ_ANNOTATION_APP_NAME = "NALEJ_APP_NAME"
    // Annotation application instance
    NALEJ_ANNOTATION_APP_INSTANCE_ID = "NALEJ_APP_INSTANCE_ID"
    // Annotation for metadata to identify the stage for these deployments
    NALEJ_ANNOTATION_STAGE_ID = "NALEJ_STAGE_ID"
    // Service name
    NALEJ_ANNOTATION_SERVICE_NAME = "NALEJ_SERVICE_NAME"
    // FQDN for a service
    NALEJ_SERVICE_FQDN = "NALEJ_SERVICE_FQDN"
    // Annotation for metadata to identify the service for these deployments
    NALEJ_ANNOTATION_SERVICE_ID = "NALEJ_SERVICE_ID"
    // Annotation for metadata to identify the service instance for these deployments
    NALEJ_ANNOTATION_SERVICE_INSTANCE_ID = "NALEJ_SERVICE_INSTANCE_ID"
    // Annotation for metadata to identify the group service
    NALEJ_ANNOTATION_SERVICE_GROUP_ID = "NALEJ_SERVICE_GROUP_ID"
    // Annotation for metadata to identify the group service
    NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID = "NALEJ_SERVICE_GROUP_INSTANCE_ID"
    // Annotation for metadata to identify endpoints for ingress
    NALEJ_ANNOTATION_INGRESS_ENDPOINT = "NALEJ_ENDPOINT"
    // Annotation for metadata to identify the security rule.
    NALEJ_ANNOTATION_SECURITY_RULE_ID = "NALEJ_SECURITY_RULE_ID"

    // TODO review this notation. It must be uppercase
    NALEJ_ANNOTATION_SERVICE_PURPOSE = "nalej-service-purpose"
    NALEJ_ANNOTATION_VALUE_DEVICE_GROUP_SERVICE = "device-group"
    NALEJ_ANNOTATION_VALUE_LOAD_BALANCER_SERVICE = "load-balancer"

    // EnvNalejAnnotationDGSecrets contains the name of the environemnt variable to store device group secrets.
    EnvNalejAnnotationDGSecrets= "NALEJ_DG_SHARED_SECRETS"

)
