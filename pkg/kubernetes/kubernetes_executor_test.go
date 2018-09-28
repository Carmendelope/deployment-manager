/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    pbApplication "github.com/nalej/grpc-application-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    "github.com/rs/zerolog/log"
)


var _ = Describe("Analysis of kubernetes structures creation", func() {

    var k8sExecutor *KubernetesExecutor
    var stop chan struct{}

    BeforeSuite(func() {
        ex, err := NewKubernetesExecutor(false)

        Expect(err).ShouldNot(HaveOccurred())
        Expect(ex).ToNot(BeNil())

        k8sExecutor = ex.(*KubernetesExecutor)

        // Run the kubernetes controller
        kontroller := NewKubernetesController(k8sExecutor)

        // Now let's start the controller
        stop = make(chan struct{})

        go kontroller.Run(1, stop)

    })

    AfterSuite(func() {
        // stop kontroller
        defer close(stop)
    })
/*
    Context("run a stage with a service that is not going to run", func(){
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
        var fragment pbConductor.DeploymentFragment

        BeforeEach(func(){
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
                AppId: &pbApplication.AppDescriptorId{OrganizationId: "test-organization", AppDescriptorId: "test-app-001"},
                Stages: []*pbConductor.DeploymentStage{&stage},

            }
        })

        It("deploys a service, second fails and waits until rollback", func(){
            err := k8sExecutor.Execute(&fragment, &stage)
            Expect(err).Should(HaveOccurred())
        })


    })
*/

    Context("run a stage with two services", func(){
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
        var fragment pbConductor.DeploymentFragment
        var deployed DeployableKubernetesStage

        port1 := pbApplication.Port{Name: "port1", ExposedPort: 3000}
        port2 := pbApplication.Port{Name: "port2", ExposedPort: 3001}

        BeforeEach(func(){

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
                AppId: &pbApplication.AppDescriptorId{OrganizationId: "test-organization", AppDescriptorId: "test-app-001"},
                Stages: []*pbConductor.DeploymentStage{&stage},
            }

        })

        It("deploys a stage and waits until completion", func(){
            deployed, err := k8sExecutor.Execute(&fragment, &stage)
            log.Info().Msgf("--2---->%+v",deployed)
            Expect(err).ShouldNot(HaveOccurred())
            //Expect(deployed).ToNot(BeNil())
        })


        AfterEach(func(){
            Expect(deployed).ToNot(BeNil())
            err := deployed.Undeploy()
            Expect(err).ShouldNot(HaveOccurred())
        })

    })
})