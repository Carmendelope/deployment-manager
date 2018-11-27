/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
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
    undeployAppCmd.Flags().StringVar(&undeployOrgId, "orgid", "", "Organization ID")
	undeployAppCmd.Flags().StringVar(&undeployAppId, "appid", "", "App Instance ID")
    undeployAppCmd.MarkFlagRequired("orgid")
    undeployAppCmd.MarkFlagRequired("appid")
}

func undeployApp() {

    conn, err := grpc.Dial(undeployDMServer, grpc.WithInsecure())

    if err != nil {
        log.Fatal().Err(err).Msgf("impossible to connect to server %s", undeployDMServer)
    }

    client := grpc_deployment_manager_go.NewDeploymentManagerClient(conn)

    request := grpc_deployment_manager_go.UndeployRequest{
        OrganizationId: undeployOrgId,
        AppInstanceId: undeployAppId,
    }

    _, err = client.Undeploy(context.Background(), &request)
    if err != nil {
        log.Error().Err(err).Msgf("error deleting app %s", undeployAppId)
        return
    }

    log.Info().Msg("Application successfully undeployed")
}
