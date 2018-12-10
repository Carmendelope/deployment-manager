package kubernetes

import (
	"fmt"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/rs/zerolog/log"
	"k8s.io/api/extensions/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	extV1Beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

type DeployableIngress struct {
	client extV1Beta1.ExtensionsV1beta1Interface
	stage * grpc_conductor_go.DeploymentStage
	targetNamespace string
	clusterPublicHostname string
}

func NewDeployableIngress(
	client *kubernetes.Clientset,
	stage grpc_conductor_go.DeploymentStage,
	targetNamespace string) * DeployableIngress {
	return &DeployableIngress{
		client: client.ExtensionsV1beta1(),
		targetNamespace: targetNamespace,
	}
}

func (di *DeployableIngress) GetId() string {
	return di.stage.StageId
}

func (di * DeployableIngress) getHTTPIngress(organizationId string, serviceId string, serviceName string, port *grpc_application_go.Port) *v1beta1.Ingress {

	paths := make([]v1beta1.HTTPIngressPath, 0)

	for _, endpoint := range port.Endpoints{
		if endpoint.Type == grpc_application_go.EndpointType_WEB || endpoint.Type == grpc_application_go.EndpointType_REST {
			toAdd := v1beta1.HTTPIngressPath{
				Path: endpoint.Path,
				Backend: v1beta1.IngressBackend{
					ServiceName: serviceName,
					ServicePort: intstr.IntOrString{IntVal: port.ExposedPort},
				},
			}
			paths = append(paths, toAdd)
		}else{
			log.Warn().Interface("port", port).Msg("Ignoring endpoint, unsupported type")
		}
	}

	if len(paths) == 0 {
		log.Debug().Str("serviceId", serviceId).Msg("service does not contain any paths")
		return nil
	}

	ingressHostname := fmt.Sprintf("%s.%s", serviceId, di.clusterPublicHostname)

	return &v1beta1.Ingress{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      fmt.Sprintf("ingress-%s", serviceId),
			Namespace: "nalej",
			Labels: map[string]string{
				"cluster":   "application",
				"component": "ingress-nginx",
			},
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
				"organizationId": organizationId,
				"serviceId": serviceId,
				"portName": port.Name,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				v1beta1.IngressRule{
					Host: ingressHostname,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: paths,
						},
					},
				},
			},
		},
	}
}

func (di * DeployableIngress) BuildIngressesForService(service *grpc_application_go.Service) []*v1beta1.Ingress {

	ingresses := make([]*v1beta1.Ingress, 0)
	for _, p := range service.ExposedPorts {
		toAdd := di.getHTTPIngress(service.OrganizationId, service.ServiceId, service.Name, p)
		ingresses = append(ingresses, toAdd)
	}
	log.Debug().Int("number", len(ingresses)).Str("serviceName", service.Name).Msg("Ingresses prepared for service")
	return ingresses
}

func (di *DeployableIngress) Build() error {
	ingresses := make(map[string] []*v1beta1.Ingress, 0)
	for _, service := range di.stage.Services {
		ingresses[service.ServiceId] = di.BuildIngressesForService(service)
	}
	return nil
}

func (di *DeployableIngress) Deploy(controller executor.DeploymentController) error {
	panic("implement me")
}

func (di *DeployableIngress) Undeploy() error {
	panic("implement me")
}


