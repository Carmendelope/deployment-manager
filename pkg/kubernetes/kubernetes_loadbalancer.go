package kubernetes

import (
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/deployment-manager/pkg/common"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/utils"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type LoadBalancerInfo struct {
	ServiceId         string
	ServiceInstanceId string
	Services []ServiceInfo
}

 type DeployableLoadBalancer struct {
	 // kubernetes Client
	 client v12.ServiceInterface
	 // Deployment metadata
	 data entities.DeploymentMetadata
	 loadBalancers []ServiceInfo
}

func NewDeployableLoadBalancer(client *kubernetes.Clientset, data entities.DeploymentMetadata) *DeployableLoadBalancer {

	return &DeployableLoadBalancer{
		client: client.CoreV1().Services(data.Namespace),
		data: data,
		loadBalancers: make([]ServiceInfo,0),
	}
}


func (dl *DeployableLoadBalancer) BuildLoadBalancerForServiceWithRule(service *grpc_application_go.ServiceInstance, rule * grpc_conductor_go.PublicSecurityRuleInstance) *apiv1.Service {

	extendedLabels := make(map[string]string,0)
	extendedLabels[utils.NALEJ_ANNOTATION_ORGANIZATION] = dl.data.OrganizationId
	extendedLabels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR] = dl.data.AppDescriptorId
	extendedLabels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID] = dl.data.AppInstanceId
	extendedLabels[utils.NALEJ_ANNOTATION_STAGE_ID] = dl.data.Stage.StageId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_ID] = service.ServiceId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID] = service.ServiceInstanceId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID] = service.ServiceGroupId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID] = service.ServiceGroupInstanceId
	extendedLabels[utils.NALEJ_ANNOTATION_SERVICE_PURPOSE] = utils.NALEJ_ANNOTATION_VALUE_LOAD_BALANCER_SERVICE

	extendedSelectors := map[string]string{
		utils.NALEJ_ANNOTATION_ORGANIZATION: dl.data.OrganizationId,
		utils.NALEJ_ANNOTATION_APP_DESCRIPTOR: dl.data.AppDescriptorId,
		utils.NALEJ_ANNOTATION_APP_INSTANCE_ID: dl.data.AppInstanceId,
		utils.NALEJ_ANNOTATION_STAGE_ID: dl.data.Stage.StageId,
		utils.NALEJ_ANNOTATION_SERVICE_ID: service.ServiceId,
		utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID: service.ServiceInstanceId,
		utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID: service.ServiceGroupId,
		utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID: service.ServiceGroupInstanceId,
	}

	found := false
	for portIndex := 0; portIndex < len(service.ExposedPorts) && !found; portIndex++{
		port := service.ExposedPorts[portIndex]
		// if there is a rule with a port and in the service definition this port has no endpoint -> Create a TCP load balancer
		if port.ExposedPort == rule.TargetPort && (port.Endpoints == nil || len(port.Endpoints) == 0 ){
			found = true

			ports := make([]apiv1.ServicePort, 0)
			ports = append(ports, apiv1.ServicePort{
				Port: port.ExposedPort,
				TargetPort: intstr.FromInt(int(rule.TargetPort)),
			})

			// Create the service
			k8sService := apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dl.data.Namespace,
					Name: fmt.Sprintf("lb%s", common.FormatName(service.Name)),
					Labels: extendedLabels,
				},
				Spec: apiv1.ServiceSpec{
					ExternalName: fmt.Sprintf("lb%s", common.FormatName(service.Name)),
					Ports: ports,
					Type: apiv1.ServiceTypeLoadBalancer,
					Selector: extendedSelectors,
				},
			}

			return &k8sService
		}
	}
	return nil
}

func (dl *DeployableLoadBalancer) GetId() string {
	return dl.data.Stage.StageId
}


func (dl *DeployableLoadBalancer) Build() error {

	log.Debug().Int("number public rules", len(dl.data.Stage.PublicRules)).Msg("Building Load Balancers")

	for _, publicRule := range dl.data.Stage.PublicRules {
		log.Debug().Interface("rule", publicRule).Msg("Checking public rule")
		for _, service := range dl.data.Stage.Services {
			log.Debug().Interface("service", service).Msg("Checking service for load balancer")
			if publicRule.TargetServiceGroupInstanceId == service.ServiceGroupInstanceId && publicRule.TargetServiceInstanceId == service.ServiceInstanceId {
				toAdd := dl.BuildLoadBalancerForServiceWithRule(service, publicRule)
				if toAdd != nil {
					log.Debug().Interface("toAdd", toAdd).Str("serviceName", service.Name).Msg("Adding new load Balancer for service")
					dl.loadBalancers = append(dl.loadBalancers, ServiceInfo{service.ServiceId, service.ServiceInstanceId, *toAdd})
				}
			}
		}
	}

	log.Debug().Interface("Load Balancers", dl.loadBalancers).Msg("Load Balancers have been build and are ready to deploy")

	return nil
}

func (dl *DeployableLoadBalancer) Deploy(controller executor.DeploymentController) error {
	for _, servInfo := range dl.loadBalancers {
		created, err := dl.client.Create(&servInfo.Service)
		if err != nil {
			log.Error().Err(err).Msgf("error creating service %s",servInfo.Service.Name)
			return err
		}
		log.Debug().Str("uid",string(created.GetUID())).Str("appInstanceID",dl.data.AppInstanceId).
			Str("serviceID", servInfo.ServiceId).Msg("add service resource to be monitored")
		res := entities.NewMonitoredPlatformResource(string(created.GetUID()),
			created.Labels[utils.NALEJ_ANNOTATION_APP_DESCRIPTOR], created.Labels[utils.NALEJ_ANNOTATION_APP_INSTANCE_ID],
			created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_GROUP_INSTANCE_ID],
			created.Labels[utils.NALEJ_ANNOTATION_SERVICE_ID], created.Labels[utils.NALEJ_ANNOTATION_SERVICE_INSTANCE_ID], "")
		controller.AddMonitoredResource(&res)
	}
	return nil
}

func (dl *DeployableLoadBalancer) Undeploy() error {
	for _, servInfo := range dl.loadBalancers {
		err := dl.client.Delete(common.FormatName(servInfo.Service.Name), metav1.NewDeleteOptions(*int64Ptr(DeleteGracePeriod)))
		if err != nil {
			log.Error().Err(err).Msgf("error deleting service %s", servInfo.Service.Name)
			return err
		}
	}
	return nil
}