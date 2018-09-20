/*
 * Copyright 2018 Nalej
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
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    pbApplication "github.com/nalej/grpc-application-go"
)


var _ = Describe("Analysis of kubernetes structures creation", func() {

    var executor *KubernetesExecutor

    BeforeSuite(func() {
        ex, err := NewKubernetesExecutor(false)

        Expect(err).ShouldNot(HaveOccurred())
        Expect(ex).ToNot(BeNil())

        executor = ex.(*KubernetesExecutor)
    })

    Context("run a deployment", func(){
        var serv pbApplication.Service
        port1 := pbApplication.Port{Name: "port1", ExposedPort: 3000}
        port2 := pbApplication.Port{Name: "port2", ExposedPort: 3001}

        BeforeEach(func(){
            serv = pbApplication.Service{
                ServiceId: "service_001",
                Name: "test-image",
                Image: "nginx:1.12",
                ExposedPorts: []*pbApplication.Port{&port1, &port2},
                Labels: map[string]string { "label1":"value1", "label2":"value2"},
                Specs: &pbApplication.DeploySpecs{Replicas: 1},
            }
        })

        It("correctly deploys the service", func(){
            err := executor.runDeployment(&serv)
            Expect(err).ShouldNot(HaveOccurred())
        })

        AfterEach(func(){
            err := executor.undeployService(&serv)
            Expect(err).ShouldNot(HaveOccurred())
        })
    })
})