/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
    "github.com/onsi/gomega"
    "github.com/onsi/ginkgo"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
    pbDeploymentManager "github.com/nalej/grpc-deployment-manager-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbApplication "github.com/nalej/grpc-application-go"
    "github.com/nalej/deployment-manager/pkg/kubernetes"
    "github.com/nalej/grpc-utils/pkg/test"
    "context"
)


var _ = ginkgo.Describe("Deployment server API", func() {
    // grpc server
    var server *grpc.Server
    // Deployment manager
    var dm *Manager
    // grpc test listener
    var listener *bufconn.Listener
    // Deployment manager client
    var client pbDeploymentManager.DeploymentManagerClient
    // Kubernetes ex
    var k8sExec *kubernetes.KubernetesExecutor



    ginkgo.BeforeSuite(func(){
        listener = test.GetDefaultListener()
        server = grpc.NewServer()
        exec, err := kubernetes.NewKubernetesExecutor(false)
        k8sExec = exec.(*kubernetes.KubernetesExecutor)
        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

        conn, err := test.GetConn(*listener)

        dm = NewManager(conn,&exec)

        test.LaunchServer(server, listener)
        // Register the service
        pbDeploymentManager.RegisterDeploymentManagerServer(server, NewHandler(dm))

        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        client = pbDeploymentManager.NewDeploymentManagerClient(conn)

       })

    ginkgo.AfterSuite(func(){
        server.Stop()
        listener.Close()
    })

    ginkgo.Context("a deployment fragment request arrives with a single stage and two services", func(){
        var request pbDeploymentManager.DeployFragmentRequest
        var fragment pbConductor.DeploymentFragment
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
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
                StageId: "stage-001",
                FragmentId: "fragment-001",
                Services: services,
            }

            fragment = pbConductor.DeploymentFragment{
                DeploymentId: "deployment-001",
                FragmentId: "fragment-001",
                AppId: &pbApplication.AppDescriptorId{AppDescriptorId:"app-001",OrganizationId:"org-001"},
                Stages: []*pbConductor.DeploymentStage{&stage},
            }

            request = pbDeploymentManager.DeployFragmentRequest{RequestId:"plan-001",Fragment: &fragment}
        })
        ginkgo.It("the new request is processed and the stages are deployed", func(){
            response, err := client.Execute(context.Background(),&request)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(response.RequestId).To(gomega.Equal(request.RequestId))
        })

        ginkgo.AfterEach(func(){
            // remove the namespace
            deployedNamespace := kubernetes.NewDeployableNamespace(k8sExec.Client,&stage,"org-001-app-001")
            err := deployedNamespace.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })
    })

    ginkgo.Context("a deployment fragment request arrives with a two stages and two services", func(){
        var request pbDeploymentManager.DeployFragmentRequest
        var fragment pbConductor.DeploymentFragment
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage1 pbConductor.DeploymentStage
        var stage2 pbConductor.DeploymentStage
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

            services1 :=[]*pbConductor.Service{&serv1}
            services2 :=[]*pbConductor.Service{&serv2}


            stage1 = pbConductor.DeploymentStage{
                StageId: "stage-001",
                FragmentId: "fragment-001",
                Services: services1,
            }

            stage2 = pbConductor.DeploymentStage{
                StageId: "stage-002",
                FragmentId: "fragment-001",
                Services: services2,
            }

            fragment = pbConductor.DeploymentFragment{
                DeploymentId: "deployment-001",
                FragmentId: "fragment-001",
                AppId: &pbApplication.AppDescriptorId{AppDescriptorId:"app-002",OrganizationId:"org-001"},
                Stages: []*pbConductor.DeploymentStage{&stage1,&stage2},
            }

            request = pbDeploymentManager.DeployFragmentRequest{RequestId:"plan-001",Fragment: &fragment}
        })
        ginkgo.It("the new request is processed and the stages are deployed", func(){
            response, err := client.Execute(context.Background(),&request)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(response.RequestId).To(gomega.Equal(request.RequestId))
        })

        ginkgo.AfterEach(func(){
            // remove the namespace
            deployedNamespace := kubernetes.NewDeployableNamespace(k8sExec.Client,&stage1,"org-001-app-002")
            err := deployedNamespace.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })
    })

})
