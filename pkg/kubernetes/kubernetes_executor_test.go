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
)


var _ = ginkgo.Describe("Analysis of kubernetes structures creation", func() {

    var k8sExecutor *KubernetesExecutor


    ginkgo.BeforeSuite(func() {
        ex, err := NewKubernetesExecutor(false)

        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        gomega.Expect(ex).ToNot(gomega.BeNil())

        k8sExecutor = ex.(*KubernetesExecutor)

    })


    ginkgo.Context("run a stage with a service that is not going to run", func(){
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
        var fragment pbConductor.DeploymentFragment
        appId := "errorapp"

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
                AppInstanceId: appId,
                Stages: []*pbConductor.DeploymentStage{&stage},
            }
        })

        ginkgo.It("deploys a service, second fails and waits until rollback", func(){
            deployed, err := k8sExecutor.Execute(&fragment, &stage)
            gomega.Expect(err).NotTo(gomega.BeNil())
            gomega.Expect(deployed).NotTo(gomega.BeNil())
        })

        ginkgo.AfterEach(func(){
            // delete the namespace
            deployedNamespace := NewDeployableNamespace(k8sExecutor.Client,&stage,"test-organization-errorapp")
            err := deployedNamespace.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })

    })


    ginkgo.Context("run a stage with two services", func(){
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
        var fragment pbConductor.DeploymentFragment
        //var deployed *executor.Deployable

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
                AppInstanceId: "app-003",
                Stages: []*pbConductor.DeploymentStage{&stage},
            }
        })

        ginkgo.It("deploys a stage and waits until completion", func(){
            deployed, err := k8sExecutor.Execute(&fragment, &stage)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(deployed).ToNot(gomega.BeNil())
        })


        ginkgo.AfterEach(func(){
            // delete the namespace
            deployedNamespace := NewDeployableNamespace(k8sExecutor.Client,&stage,"test-organization-test-app-001")
            err := deployedNamespace.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })

    })

})