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

package cmd

import (
	"context"
	"github.com/nalej/grpc-deployment-manager-go"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// Deployment Manager IP
var undeployDMServer string

// Organization ID
var undeployOrgId string

// App Instance ID
var undeployAppId string

var undeployAppCmd = &cobra.Command{
	Use:   "undeploy",
	Short: "Undeploy an application",
	Long:  `Undeploy an application`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		undeployApp()
	},
}

func init() {
	RootCmd.AddCommand(undeployAppCmd)
	undeployAppCmd.Flags().StringVar(&undeployDMServer, "server", "localhost:5200", "Deployment Manager server URL")
	undeployAppCmd.Flags().StringVar(&undeployOrgId, "orgId", "", "Organization ID")
	undeployAppCmd.Flags().StringVar(&undeployAppId, "appId", "", "App Instance ID")
	undeployAppCmd.MarkFlagRequired("orgId")
	undeployAppCmd.MarkFlagRequired("appId")
}

func undeployApp() {

	conn, err := grpc.Dial(undeployDMServer, grpc.WithInsecure())

	if err != nil {
		log.Fatal().Err(err).Msgf("impossible to connect to server %s", undeployDMServer)
	}

	client := grpc_deployment_manager_go.NewDeploymentManagerClient(conn)

	request := grpc_deployment_manager_go.UndeployRequest{
		OrganizationId: undeployOrgId,
		AppInstanceId:  undeployAppId,
	}

	_, err = client.Undeploy(context.Background(), &request)
	if err != nil {
		log.Error().Err(err).Msgf("error deleting app %s", undeployAppId)
		return
	}

	log.Info().Msg("Application successfully undeployed")
}
