/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    "github.com/onsi/ginkgo"
    "github.com/onsi/gomega"
    pbApplication "github.com/nalej/grpc-application-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    monitor2 "github.com/nalej/deployment-manager/pkg/monitor"
    "google.golang.org/grpc"
    "github.com/nalej/deployment-manager/pkg/executor"
)

var ConnectorAddress string

var _ = ginkgo.Describe("Analysis of kubernetes structures creation", func() {

    var k8sExecutor *KubernetesExecutor
    var monitor *monitor2.MonitorHelper


    ginkgo.BeforeSuite(func() {
        ex, err := NewKubernetesExecutor(false)

        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        gomega.Expect(ex).ToNot(gomega.BeNil())

        k8sExecutor = ex.(*KubernetesExecutor)

        // Instantiate a new monitor
        ConnectorAddress = "localhost:5000"
        conn, err := grpc.Dial(ConnectorAddress, grpc.WithInsecure())
        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

        monitor = monitor2.NewMonitorHelper(conn)

    })


    ginkgo.Context("run a stage with a service that is not going to run", func(){
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
        var fragment pbConductor.DeploymentFragment
        var preDeployed executor.Deployable
        namespace := "test-app-single"

        ginkgo.BeforeEach(func(){
            serv1 = pbApplication.Service{
                ServiceId: "service_001",
                Name: "test-image-1",
                Image: "nginx:1.12",
                Labels: map[string]string { "label1":"value1", "label2":"value2"},
                Specs: &pbApplication.DeploySpecs{Replicas: 1},
            }

            serv2 = pbApplication.Service{
                ServiceId: "service_002_error",
                Name: "test-image-2",
                Image: "errorimage:1.12",
                Labels: map[string]string { "label1":"value1"},
                Specs: &pbApplication.DeploySpecs{Replicas: 2},
            }

            services :=[]*pbConductor.Service{&serv1,&serv2}

            stage = pbConductor.DeploymentStage{
                StageId: "error_stage_001",
                Services: services,
            }
            fragment = pbConductor.DeploymentFragment{
                FragmentId: "fragment_001",
                DeploymentId: "deployment_001",
                AppInstanceId: "errorapp",
                OrganizationId: "test-organization",

                Stages: []*pbConductor.DeploymentStage{&stage},
            }
        })

        ginkgo.It("deploys a service, second fails and waits until rollback", func(){
            aux, err := k8sExecutor.PrepareEnvironmentForDeployment(&fragment, namespace, monitor)
            preDeployed = aux
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            toDeploy, err := k8sExecutor.BuildNativeDeployable(&stage, namespace)
            gomega.Expect(toDeploy).NotTo(gomega.BeNil())
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            err = k8sExecutor.DeployStage(toDeploy, &fragment, &stage, monitor)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(err).NotTo(gomega.BeNil())

        })

        ginkgo.AfterEach(func(){
            err := preDeployed.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })

    })


    ginkgo.Context("run a stage with two services", func(){
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
        var fragment pbConductor.DeploymentFragment
        var preDeployed executor.Deployable
        namespace := "test-app-double"

        port1 := pbApplication.Port{Name: "port1", ExposedPort: 3000}
        port2 := pbApplication.Port{Name: "port2", ExposedPort: 3001}

        ginkgo.BeforeEach(func(){

            serv1 = pbApplication.Service{
                ServiceId: "service_001",
                Name: "test-image-1",
                Image: "nginx:1.12",
                ExposedPorts: []*pbApplication.Port{&port1, &port2},
                Labels: map[string]string { "label1":"value1", "label2":"value2"},
                Specs: &pbApplication.DeploySpecs{Replicas: 1},
            }

            serv2 = pbApplication.Service{
                ServiceId: "service_002",
                Name: "test-image-2",
                Image: "nginx:1.12",
                ExposedPorts: []*pbApplication.Port{&port1, &port2},
                Labels: map[string]string { "label1":"value1"},
                Specs: &pbApplication.DeploySpecs{Replicas: 2},
            }

            services :=[]*pbConductor.Service{&serv1,&serv2}

            stage = pbConductor.DeploymentStage{
                StageId: "stage_001",
                Services: services,
            }
            fragment = pbConductor.DeploymentFragment{
                FragmentId: "fragment_001",
                DeploymentId: "deployment_001",
                AppInstanceId: "test-app-001",
                Stages: []*pbConductor.DeploymentStage{&stage},
                OrganizationId: "test-organization",
            }
        })

        ginkgo.It("deploys a stage and waits until completion", func(){
            aux, err := k8sExecutor.PrepareEnvironmentForDeployment(&fragment, namespace, monitor)
            preDeployed = aux
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            toDeploy, err := k8sExecutor.BuildNativeDeployable(&stage, namespace)
            gomega.Expect(toDeploy).NotTo(gomega.BeNil())
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            err = k8sExecutor.DeployStage(toDeploy, &fragment, &stage, monitor)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(err).NotTo(gomega.BeNil())

        })


        ginkgo.AfterEach(func(){
            err := preDeployed.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })

    })

})