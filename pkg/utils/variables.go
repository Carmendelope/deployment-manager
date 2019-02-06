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

    // Annotation to identify services deployed by nalej
    NALEJ_ANNOTATION_SERVICE_ID = "nalej-service"

    // Annotation for metadata to identify the stage for these deployments
    NALEJ_ANNOTATION_STAGE_ID = "nalej-stage"
    // Annotation for metadata to identify the stage for these deployments
    NALEJ_ANNOTATION_INSTANCE_ID = "nalej-instance"
    // Annotation for metadata to identify the group service
    NALEJ_ANNOTATION_SERVICE_GROUP_ID = "nalej-service-group"
    // Annotation for metadata to identify endpoints for ingress
    NALEJ_ANNOTATION_INGRESS_ENDPOINT = "nalej-endpoint"


    // Environment variable indicating the conductor address
    IT_CONDUCTOR_ADDRESS = "IT_CONDUCTOR_ADDRESS"
)
