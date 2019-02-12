/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 */

package utils

// Collection of variable names to be used in the project

const (
    // Address of the remote manager cluster in charge of controlling this deployment manager.
    MANAGER_ClUSTER_IP = "MANAGER_CLUSTER_IP"

    // Port of the remote manager cluster
    MANAGER_CLUSTER_PORT = "MANAGER_CLUSTER_PORT"

    // Name of the variable containing the cluster id.
    CLUSTER_ID = "CLUSTER_ID"


    // Annotation for the organization
    NALEJ_ANNOTATION_ORGANIZATION = "nalej-organization"
    // Annotation application descriptor
    NALEJ_ANNOTATION_APP_DESCRIPTOR = "nalej-app-descriptor"
    // Annotation application instance
    NALEJ_ANNOTATION_APP_INSTANCE_ID = "nalej-app-instance-id"
    // Annotation for metadata to identify the stage for these deployments
    NALEJ_ANNOTATION_STAGE_ID = "nalej-stage-id"
    // Annotation for metadata to identify the service for these deployments
    NALEJ_ANNOTATION_SERVICE_ID = "nalej-service-id"
    // Annotation for metadata to identify the group service
    NALEJ_ANNOTATION_SERVICE_GROUP_ID = "nalej-service-group-id"
    // Annotation for metadata to identify the group service
    NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID = "nalej-service-group-instance-id"
    // Annotation for metadata to identify endpoints for ingress
    NALEJ_ANNOTATION_INGRESS_ENDPOINT = "nalej-endpoint"


    // Environment variable indicating the conductor address
    IT_CONDUCTOR_ADDRESS = "IT_CONDUCTOR_ADDRESS"
)
