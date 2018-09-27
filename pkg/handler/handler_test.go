/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
    . "github.com/onsi/gomega"
    . "github.com/onsi/ginkgo"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
    pbDeploymentManager "github.com/nalej/grpc-deployment-manager-go"
    pbConductor "github.com/nalej/grpc-conductor-go"
    pbApplication "github.com/nalej/grpc-application-go"
    "github.com/nalej/deployment-manager/tools"
    "github.com/nalej/deployment-manager/pkg/kubernetes"

    "context"
    executor "github.com/nalej/deployment-manager/pkg/executor"
)

const TestPort = 4000

var _ = Describe("Deployment server API", func() {
    // grpc server
    var server *grpc.Server
    // Deployment manager
    var dm *Manager
    // grpc test listener
    var listener *bufconn.Listener
    // Deployment manager client
    var client pbDeploymentManager.DeploymentManagerClient
    // Channel for kubernetes controller
    var stop chan struct{}
    // Kubernetes executor
    var executor executor.Executor

    BeforeSuite(func(){
        listener = tools.GetDefaultListener()
        server = grpc.NewServer()
        exec, err := kubernetes.NewKubernetesExecutor(false)
        Expect(err).ShouldNot(HaveOccurred())
        executor = exec.(*kubernetes.KubernetesExecutor)
        dm = NewManager(&executor)
        conn, err := tools.GetConn(*listener)
        tools.LaunchServer(server, listener)
        // Register the service
        pbDeploymentManager.RegisterDeploymentManagerServer(server, NewHandler(dm))

        Expect(err).ShouldNot(HaveOccurred())
        client = pbDeploymentManager.NewDeploymentManagerClient(conn)

        // Run the kubernetes controller
        kontroller := kubernetes.NewKubernetesController(executor.(*kubernetes.KubernetesExecutor))

        // Now let's start the controller
        stop = make(chan struct{})

        go kontroller.Run(1, stop)
    })

    AfterSuite(func(){
        server.Stop()
        listener.Close()
        // stop kontroller
        defer close(stop)
    })

    Context("a deployment plan request arrives with a single stage and two services", func(){
        var request pbDeploymentManager.DeployPlanRequest
        var plan pbConductor.DeploymentPlan
        var serv1 pbApplication.Service
        var serv2 pbApplication.Service
        var stage pbConductor.DeploymentStage
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
                DeploymentId: "deployment_001",
                Services: services,
            }

            plan = pbConductor.DeploymentPlan{
                DeploymentId: "plan_001",
                Stages: []*pbConductor.DeploymentStage{&stage},
            }

            request = pbDeploymentManager.DeployPlanRequest{RequestId:"plan_001",Plan: &plan}
        })
        It("the new request is processed and the stages are deployed", func(){
            response, err := client.Execute(context.Background(),&request)
            Expect(err).ShouldNot(HaveOccurred())
            Expect(response.RequestId).To(Equal(request.RequestId))
        })

        AfterEach(func(){
            err := executor.UndeployService(&serv1)
            Expect(err).ShouldNot(HaveOccurred())
            err = executor.UndeployService(&serv2)
            Expect(err).ShouldNot(HaveOccurred())
        })
    })

})
