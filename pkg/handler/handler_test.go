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

package handler

// Tests have to be updated to use emulated secure login.
/*
var _ = ginkgo.Describe("Deployment server API", func() {
    var isReady bool
    // Conductor address
    var conductorAddress string
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
        isReady = false
        if utils.RunIntegrationTests() {
            conductorAddress = os.Getenv(utils.IT_CONDUCTOR_ADDRESS)
            if conductorAddress != "" {
                isReady = true
            }
        }

        if !isReady {
            return
        }


        listener = test.GetDefaultListener()
        server = grpc.NewServer()
        exec, err := kubernetes.NewKubernetesExecutor(false)
        k8sExec = exec.(*kubernetes.KubernetesExecutor)
        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

        localConn, err := test.GetConn(*listener)
        conn, err := grpc.Dial(conductorAddress, grpc.WithInsecure())
        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

        dm = NewManager(conn, &exec)

        test.LaunchServer(server, listener)
        // Register the service
        pbDeploymentManager.RegisterDeploymentManagerServer(server, NewHandler(dm))

        gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        client = pbDeploymentManager.NewDeploymentManagerClient(localConn)

       })

    ginkgo.AfterSuite(func(){
        if !isReady {
            return
        }
        server.Stop()
        listener.Close()
    })

    ginkgo.Context("a deployment fragment request arrives with a single stage and two services", func(){
        var request pbDeploymentManager.DeploymentFragmentRequest
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
                AppInstanceId: "app-001",
                OrganizationId: "org-001",
                Stages: []*pbConductor.DeploymentStage{&stage},
            }

            request = pbDeploymentManager.DeploymentFragmentRequest{RequestId:"plan-001",Fragment: &fragment, ZtNetworkId: "ztNetwork1"}
        })
        ginkgo.It("the new request is processed and the stages are deployed", func(){
            if !isReady {
                ginkgo.Skip("no integration test is set")
            }
            response, err := client.Execute(context.Background(),&request)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(response.RequestId).To(gomega.Equal(request.RequestId))
        })

        ginkgo.AfterEach(func(){
            // remove the namespace
            deployedNamespace := kubernetes.NewDeployableNamespace(k8sExec.Client,stage.FragmentId,"org-001-app-001")
            err := deployedNamespace.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })
    })

    ginkgo.Context("a deployment fragment request arrives with a two stages and two services", func(){
        var request pbDeploymentManager.DeploymentFragmentRequest
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
                AppInstanceId: "app-002",
                OrganizationId: "org-001",
                Stages: []*pbConductor.DeploymentStage{&stage1,&stage2},
            }

            request = pbDeploymentManager.DeploymentFragmentRequest{RequestId:"plan-001",Fragment: &fragment, ZtNetworkId: "ztNetwork1"}
        })
        ginkgo.It("the new request is processed and the stages are deployed", func(){
            if !isReady {
                ginkgo.Skip("no integration test is set")
            }
            response, err := client.Execute(context.Background(),&request)
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
            gomega.Expect(response.RequestId).To(gomega.Equal(request.RequestId))
        })

        ginkgo.AfterEach(func(){
            // remove the namespace
            deployedNamespace := kubernetes.NewDeployableNamespace(k8sExec.Client,stage1.FragmentId,"org-001-app-002")
            err := deployedNamespace.Undeploy()
            gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
        })
    })

})
*/
