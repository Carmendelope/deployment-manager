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
package kubernetes

import (
	"fmt"
	"github.com/nalej/deployment-manager/internal/entities"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-conductor-go"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
)

var _ = ginkgo.Describe("Kubernetes ConfigMap tests", func() {

	var client *DeployableConfigMaps
	var deploymentClient *DeployableDeployments
	var configFiles []*grpc_application_go.ConfigFile
	var service *grpc_conductor_go.ServiceInstance

	ginkgo.BeforeSuite(func() {
		log.Debug().Msg("BeforeSuite")
		client = NewDeployableConfigMapsForTest(entities.DeploymentMetadata{
			Namespace: "Namespace",
		})
		deploymentClient = NewDeployableDeploymentForTest()

		configFiles = make([]*grpc_application_go.ConfigFile, 0)

		conf1 := grpc_application_go.ConfigFile{
			ConfigFileId: "1",
			Content:      []byte{0x00},
			MountPath:    fmt.Sprintf("/etc/config/file1.txt"),
		}
		conf2 := grpc_application_go.ConfigFile{
			ConfigFileId: "2",
			Content:      []byte{0x00},
			MountPath:    fmt.Sprintf("/opt/files/file2.txt"),
		}
		conf3 := grpc_application_go.ConfigFile{
			ConfigFileId: "3",
			Content:      []byte{0x00},
			MountPath:    fmt.Sprintf("/opt/files/file3.txt"),
		}
		configFiles = append(configFiles, &conf1, &conf2, &conf3)

	})

	ginkgo.AfterSuite(func() {
		log.Debug().Msg("AfterSuite")
	})

	ginkgo.It("Should be able to create a configMap", func() {
		log.Debug().Msg("IT")

		service = &grpc_conductor_go.ServiceInstance{ServiceName: "service_id"}

		config := client.generateConsolidateConfigMap(service, configFiles)
		gomega.Expect(len(config.BinaryData)).Should(gomega.Equal(len(configFiles)))

		volumes, volumesMount := deploymentClient.generateAllVolumes("service_id", "service_instance_id", configFiles)
		gomega.Expect(len(volumesMount)).Should(gomega.Equal(2))
		gomega.Expect(len(volumes)).Should(gomega.Equal(2))

	})

})
