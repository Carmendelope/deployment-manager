/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */

package kubernetes

import (
	"fmt"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type DeployableStorage struct {
	client          coreV1.PersistentVolumeClaimInterface
	stage           *grpc_conductor_go.DeploymentStage
	targetNamespace string
	class    		string
	pvcs      		map[string][]*v1.PersistentVolumeClaim
}

func NewDeployableStorage(
	client *kubernetes.Clientset,
	stage *grpc_conductor_go.DeploymentStage,
	targetNamespace string) *DeployableStorage {

	sc := ""
	return &DeployableStorage{
		client:          client.CoreV1().PersistentVolumeClaims(targetNamespace),
		stage:           stage,
		targetNamespace: targetNamespace,
		class:			sc,
		pvcs:      make(map[string][]*v1.PersistentVolumeClaim, 0),
	}
}

func (ds*DeployableStorage) GetId() string {
	return ds.stage.StageId
}


func (ds*DeployableStorage) generatePVC(storageId string, storage *grpc_application_go.Storage) *v1.PersistentVolumeClaim {


	if storage.Size == 0 {
		storage.Size = DefaultStorageAllocationSize
	}
	sizeQuantity := resource.NewQuantity(storage.Size,resource.BinarySI)
	return &v1.PersistentVolumeClaim{
		TypeMeta:   v12.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:         storageId,
			Namespace:    ds.targetNamespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce,},
			StorageClassName: &ds.class,
			Resources: v1.ResourceRequirements{ Requests:v1.ResourceList{v1.ResourceStorage:*sizeQuantity}},

		},
	}
}

//This function returns the storage class name based on the storage type and cluster environment
func (ds *DeployableStorage) GetStorageClass(stype grpc_application_go.StorageType, ctype string) string{
    sc := ""
    // Get the right class based on cluster hosting environment
    switch(ctype) {
    case "azure":
          // use azure provided sotrage class. By default managed-premium is local replicated PVs
          // TODO: if we want we can create nalej storage class for azure to allocate PVs from non replicated pool
        switch(stype) {
        case grpc_application_go.StorageType_CLUSTER_LOCAL:
            sc = "managed-premium"
        case grpc_application_go.StorageType_CLUSTER_REPLICA:
            sc = "managed-premium"
        default: sc = ""
        }
    case "das":
        switch(stype) {
        case grpc_application_go.StorageType_CLUSTER_LOCAL:
            sc = "nalej-sc-local"
        case grpc_application_go.StorageType_CLUSTER_REPLICA:
            sc = "nalej-sc-local-replica"
        default: sc = ""
        }
    default:
        sc = ""
    }
    return sc
}

// This function returns an array in case we support other Secrets in the future.
func (ds*DeployableStorage) BuildStorageForServices(service *grpc_application_go.Service) []*v1.PersistentVolumeClaim {
	if service.Storage == nil{
		return nil
	}
	pvcs := make([]*v1.PersistentVolumeClaim, 0)
	for index, storage := range service.Storage {
		if storage.Type == grpc_application_go.StorageType_EPHEMERAL {
			continue
		}
		// TODO: Currently handle only cluster_local type, other types in plan phase.
		ds.class = ds.GetStorageClass(storage.Type, common.CLUSTER_ENV)
		if ds.class == "" {
			log.Error().Str("serviceName", service.Name).Str("storage type: ", grpc_application_go.StorageType_name[int32(storage.Type)]).Str("or cluster environment: ", common.CLUSTER_ENV).Msg("not supported ")
			continue
		}
		// construct PVC ID - based on serviceId and storage Index
		//pvcId := fmt.Sprintf("%s-%s-1%d",service.Name,service.ServiceId,index)
		pvcId := common.GetNamePVC(service.AppDescriptorId,service.ServiceId,fmt.Sprintf("%d",index))
		toAdd := ds.generatePVC(pvcId, storage)
		pvcs = append(pvcs, toAdd)
	}
	log.Debug().Interface("number", len(pvcs)).Str("serviceName", service.Name).Msg("Storage prepared for service")
	if len(pvcs) > 0 {
		return pvcs
	}
	return nil
}



// storage should be build only once when platform application cluster modules are deployed.
func (ds*DeployableStorage) Build() error {
	for _, service := range ds.stage.Services {
		toAdd := ds.BuildStorageForServices(service)
		if toAdd != nil && len(toAdd) > 0 {
			ds.pvcs[service.ServiceId] = toAdd
		}
	}
	log.Debug().Interface("Storage", ds.pvcs).Msg("Storage have been build and are ready to deploy")
	return nil
}

func (ds*DeployableStorage) Deploy(controller executor.DeploymentController) error {
	numCreated := 0
	for serviceId, pvcs := range ds.pvcs {
		for _, toCreate := range pvcs {
			log.Debug().Interface("toCreate", toCreate).Msg("creating Persistence Storage ")
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

func (ds*DeployableStorage) Undeploy() error {
	deleted := 0
	for serviceId, pvcs := range ds.pvcs {
		for _, toDelete := range pvcs {
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


