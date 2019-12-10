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

package kubernetes

import (
	"context"
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-storage-fabric-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"strings"
	"time"
)

const (
	STORAGE_CLASS_AZURE_CLUSTER_LOCAL   = "managed-premium"
	STORAGE_CLASS_AZURE_CLUSTER_REPLICA = "managed-premium"
	STORAGE_CLASS_NALEJ_CLUSTER_LOCAL   = "nalej-sc-local"
	STORAGE_CLASS_NALEJ_CLUSTER_REPLICA = "nalej-sc-local-replica"

	DefaultTimeout  = time.Minute
	DefaultReplicas = 3
)

type DeployableStorage struct {
	client   coreV1.PersistentVolumeClaimInterface
	data     entities.DeploymentMetadata
	class    string
	nodes    int
	pvcs     map[string][]*v1.PersistentVolumeClaim
	sfClient grpc_storage_fabric_go.StorageClassClient
}

func NewDeployableStorage(
	client *kubernetes.Clientset,
	data entities.DeploymentMetadata,
	sfClient grpc_storage_fabric_go.StorageClassClient) *DeployableStorage {

	sc := ""
	// get number of nodes in a cluster. Log message if storage type is "cluster replica" and nodes < 3
	numNodes := 0
	nodes, err := client.CoreV1().Nodes().List(metaV1.ListOptions{Limit: int64(3)})
	if err == nil {
		numNodes = len(nodes.Items)
	}
	return &DeployableStorage{
		client:   client.CoreV1().PersistentVolumeClaims(data.Namespace),
		data:     data,
		nodes:    numNodes,
		class:    sc,
		pvcs:     make(map[string][]*v1.PersistentVolumeClaim, 0),
		sfClient: sfClient,
	}
}

func (ds *DeployableStorage) GetId() string {
	return ds.data.Stage.StageId
}

func (ds *DeployableStorage) generatePVC(storageId string, service *grpc_conductor_go.ServiceInstance,
	storage *grpc_application_go.Storage) *v1.PersistentVolumeClaim {

	if storage.Size == 0 {
		storage.Size = DefaultStorageAllocationSize
	}
	sizeQuantity := resource.NewQuantity(storage.Size, resource.BinarySI)
	return &v1.PersistentVolumeClaim{
		TypeMeta: v12.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      storageId,
			Namespace: ds.data.Namespace,
			Labels: map[string]string{
				utils.NALEJ_ANNOTATION_DEPLOYMENT_FRAGMENT:       ds.data.FragmentId,
				utils.NALEJ_ANNOTATION_ORGANIZATION_ID:           ds.data.OrganizationId,
				utils.NALEJ_ANNOTATION_APP_DESCRIPTOR:            ds.data.AppDescriptorId,
				utils.NALEJ_ANNOTATION_APP_INSTANCE_ID:           ds.data.AppInstanceId,
				utils.NALEJ_ANNOTATION_STAGE_ID:                  ds.data.Stage.StageId,
				utils.NALEJ_ANNOTATION_SERVICE_ID:                storageId,
				utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID:       service.ServiceInstanceId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID:          service.ServiceGroupId,
				utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID: service.ServiceGroupInstanceId,
				utils.NALEJ_ANNOTATION_STORAGE_TYPE:              storage.Type.String(),
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			StorageClassName: &ds.class,
			Resources:        v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceStorage: *sizeQuantity}},
		},
	}
}

//This function returns the storage class name based on the storage type and cluster environment
func (ds *DeployableStorage) GetStorageClass(stype grpc_application_go.StorageType, organizationId string, appInstanceId string, serviceInstanceId string) string {
	sc := ""
	// Get the right class based on cluster hosting environment
	switch config.GetConfig().TargetPlatform {
	case grpc_installer_go.Platform_AZURE:
		// use azure provided storage class. By default managed-premium is local replicated PVs
		// TODO: if we want we can create nalej storage class for azure to allocate PVs from non replicated pool
		switch stype {
		case grpc_application_go.StorageType_CLUSTER_LOCAL:
			sc = STORAGE_CLASS_AZURE_CLUSTER_LOCAL
		case grpc_application_go.StorageType_CLUSTER_REPLICA:
			sc = STORAGE_CLASS_AZURE_CLUSTER_REPLICA
		case grpc_application_go.StorageType_EXPERIMENTAL_CLUSTER_REPLICA:
			sc = ds.createExperimentalStorageClassName(organizationId, appInstanceId, serviceInstanceId)
		default:
			sc = ""
		}
	case grpc_installer_go.Platform_MINIKUBE:
		switch stype {
		case grpc_application_go.StorageType_CLUSTER_LOCAL:
			sc = STORAGE_CLASS_NALEJ_CLUSTER_LOCAL
		case grpc_application_go.StorageType_CLUSTER_REPLICA:
			if ds.nodes < 3 {
				log.Debug().Interface("Nodes", ds.nodes).Msg("Less than minimum 3 required for Storage Type CLUSTER_REPLICA")
			}
			sc = STORAGE_CLASS_NALEJ_CLUSTER_REPLICA
		default:
			sc = ""
		}
	default:
		sc = ""
	}
	return sc
}

// This function returns an array in case we support other Secrets in the future.
func (ds *DeployableStorage) BuildStorageForServices(service *grpc_conductor_go.ServiceInstance) []*v1.PersistentVolumeClaim {
	if service.Storage == nil {
		return nil
	}
	pvcs := make([]*v1.PersistentVolumeClaim, 0)
	for index, storage := range service.Storage {
		if storage.Type == grpc_application_go.StorageType_EPHEMERAL {
			continue
		}
		// TODO: Currently handle only cluster_local type, other types in plan phase.
		if storage.Type != grpc_application_go.StorageType_CLUSTER_LOCAL &&
			storage.Type != grpc_application_go.StorageType_CLUSTER_REPLICA &&
			storage.Type != grpc_application_go.StorageType_EXPERIMENTAL_CLUSTER_REPLICA {
			// TODO:Ideally we should return error and user should know why
			log.Error().Str("serviceName", service.ServiceName).Str("StorageType", storage.Type.String()).Msg("storage not supported ")
			// service will fail if we continue, as no PVC can be bound
			continue
		}
		ds.class = ds.GetStorageClass(storage.Type, service.OrganizationId, service.AppInstanceId, service.ServiceInstanceId)
		if ds.class == "" {

			log.Error().Str("serviceName", service.ServiceName).Str("storage type: ", grpc_application_go.StorageType_name[int32(storage.Type)]).Str("or cluster environment: ",
				grpc_installer_go.Platform_name[int32(config.GetConfig().TargetPlatform)]).Msg("not supported ")
			continue
		}
		// construct PVC ID - based on serviceId and storage Index
		pvcId := common.GeneratePVCName(service.ServiceGroupInstanceId, service.ServiceId, fmt.Sprintf("%d", index))
		toAdd := ds.generatePVC(pvcId, service, storage)
		pvcs = append(pvcs, toAdd)
	}
	log.Debug().Interface("number", len(pvcs)).Str("serviceName", service.ServiceName).Msg("Storage prepared for service")
	if len(pvcs) > 0 {
		return pvcs
	}
	return nil
}

// storage should be build only once when platform application cluster modules are deployed.
func (ds *DeployableStorage) Build() error {
	for _, service := range ds.data.Stage.Services {
		toAdd := ds.BuildStorageForServices(service)
		if toAdd != nil && len(toAdd) > 0 {
			ds.pvcs[service.ServiceId] = toAdd
		}
	}
	log.Debug().Interface("Storage", ds.pvcs).Msg("Storage have been build and are ready to deploy")
	return nil
}

func (ds *DeployableStorage) Deploy(controller executor.DeploymentController) error {
	numCreated := 0
	for serviceId, pvcs := range ds.pvcs {
		for _, toCreate := range pvcs {
			log.Debug().Interface("toCreate", toCreate).Msg("creating Persistence Storage ")

			// if the pvc contains the label NALEJ_ANNOTATION_STORAGE_TYPE: StorageType_EXPERIMENTAL_CLUSTER_REPLICA -> create it
			stoType, exists := toCreate.Labels[utils.NALEJ_ANNOTATION_STORAGE_TYPE]
			if exists && stoType == grpc_application_go.StorageType_EXPERIMENTAL_CLUSTER_REPLICA.String() {
				go ds.createExperimentalStorage(toCreate)
			}

			created, err := ds.client.Create(toCreate)
			if err != nil {
				log.Error().Err(err).Interface("toCreate", toCreate).Msg("cannot create Persistence Storage")
				return err
			}
			log.Debug().Str("serviceId", serviceId).Str("uid", string(created.GetUID())).Msg("Persistence Storage has been created")
			numCreated++
		}
	}
	log.Debug().Int("created", numCreated).Msg("Storage have been created")
	return nil
}

func (ds *DeployableStorage) Undeploy() error {
	deleted := 0
	for serviceId, pvcs := range ds.pvcs {
		for _, toDelete := range pvcs {

			// if the pvc contains the label NALEJ_ANNOTATION_STORAGE_TYPE: StorageType_EXPERIMENTAL_CLUSTER_REPLICA -> create it
			stoType, exists := toDelete.Labels[utils.NALEJ_ANNOTATION_STORAGE_TYPE]
			if exists && stoType == grpc_application_go.StorageType_EXPERIMENTAL_CLUSTER_REPLICA.String() {
				go ds.removeExperimentalStorage(toDelete)
			}

			err := ds.client.Delete(toDelete.Name, metaV1.NewDeleteOptions(DeleteGracePeriod))
			if err != nil {
				log.Error().Str("serviceId", serviceId).Interface("toDelete", toDelete).Msg("cannot delete Persistence Storage")
				return err

			}
			log.Debug().Str("serviceId", serviceId).Str("Name", toDelete.Name).Msg("Persistence Storage has been deleted")
		}
		deleted++
	}
	log.Debug().Int("deleted", deleted).Msg("Persistence Storage have been deleted")
	return nil
}

func (ds *DeployableStorage) createExperimentalStorageClassName(organizationId string, appInstanceId string, serviceInstanceId string) string{
	return fmt.Sprintf("%s%s%s", strings.Replace(organizationId, "-", "", -1),
		strings.Replace(appInstanceId, "-", "", -1),
		strings.Replace(serviceInstanceId, "-", "", -1))
}

// createExperimentalStorage call to storage fabric to create an experimental Storage
func (ds *DeployableStorage) createExperimentalStorage(pvc *v1.PersistentVolumeClaim) error {
	serviceInstanceId, exists := pvc.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID]
	if !exists {
		return derrors.NewInvalidArgumentError("serviceInstanceId not found creating Experimental Storage")
	}

	storageClassName := ds.createExperimentalStorageClassName(ds.data.OrganizationId, ds.data.AppInstanceId, serviceInstanceId)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	_, err := ds.sfClient.Create(ctx, &grpc_storage_fabric_go.CreateStorageClassRequest{
		OrganizationId:    ds.data.OrganizationId,
		AppInstanceId:     ds.data.AppInstanceId,
		ServiceInstanceId: serviceInstanceId,
		StorageClassName:  storageClassName,
		Replicas:          DefaultReplicas,
		Size:              int64(pvc.Spec.Size()),
	})

	if err != nil {
		log.Err(err).Msg("error creating experimental storage")
	}

	return nil
}

// removeExperimentalStorage call to storage fabric to remove an experimental Storage
func (ds *DeployableStorage) removeExperimentalStorage(pvc *v1.PersistentVolumeClaim) error {
	// get the organizationId, appInstanceId and serviceInstanceId to generate the experimentalClass
	organizationId, exists := pvc.Labels[utils.NALEJ_ANNOTATION_ORGANIZATION_ID]
	if !exists {
		return derrors.NewInvalidArgumentError("organizationId not found creating Experimental Storage")
	}
	appInstanceId, exists :=  pvc.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID]
	if !exists {
		return derrors.NewInvalidArgumentError("appInstanceId not found creating Experimental Storage")
	}
	serviceInstanceId, exists := pvc.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID]
	if !exists {
		return derrors.NewInvalidArgumentError("serviceInstanceId not found creating Experimental Storage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	_, err := ds.sfClient.Remove(ctx, &grpc_storage_fabric_go.StorageClassId{
		StorageClassName:  ds.createExperimentalStorageClassName(organizationId, appInstanceId, serviceInstanceId),
	})

	if err != nil {
		log.Err(err).Msg("error removing experimental storage")
	}

	return nil
}