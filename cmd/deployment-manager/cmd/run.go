/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package cmd

import (
	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/service"
	"github.com/nalej/grpc-application-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run deployment manager",
	Long:  "Run deployment manager service with... and with...",
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		Run()
	},
}

func init() {
	// UNIX Time is faster and smaller than most timestamps
	// If you set zerolog.TimeFieldFormat to an empty string,
	// logs will write with UNIX time
	zerolog.TimeFieldFormat = ""

	RootCmd.AddCommand(runCmd)
	runCmd.Flags().Uint32P("port", "p", 5200, "port where deployment manager listens to")
	runCmd.Flags().Uint32("metricsPort", 5201, "port for HTTP metrics endpoint")
	runCmd.Flags().BoolP("local", "l", false, "indicate local k8s instance")
	//runCmd.Flags().StringP("clusterAPIAddress", "c", "localhost:5500", "conductor address e.g.: 192.168.1.4:5000")
	runCmd.Flags().StringP("networkMgrAddress", "n", "localhost:8000", "network address e.g.: 192.168.1.4:8000")
	runCmd.Flags().StringP("depMgrAddress", "d", "localhost:5200", "deployment manager address e.g.: deployment-manager.nalej:5200")
	runCmd.Flags().String("managementHostname", "", "Hostname of the management cluster")
	runCmd.Flags().String("clusterAPIHostname", "", "Hostname of the cluster API on the management cluster")
	runCmd.Flags().Uint32("clusterAPIPort", 8000, "Port where the cluster API is listening")
	runCmd.Flags().Bool("useTLSForClusterAPI", true, "Use TLS to connect to the Cluster API")
	runCmd.Flags().String("loginHostname", "", "Hostname of the login service")
	runCmd.Flags().Uint32("loginPort", 31683, "port where the login service is listening")
	runCmd.Flags().Bool("useTLSForLogin", true, "Use TLS to connect to the Login API")
	runCmd.Flags().String("clusterPublicHostname", "", "Cluster Public Hostname for the ingresses")
	runCmd.Flags().StringP("email", "e", "admin@nalej.com", "email address")
	runCmd.Flags().StringP("password", "w", "Passw0rd666", "password")
	runCmd.Flags().StringP("dns", "s", "", "List of dns ips separated by commas")
	runCmd.Flags().String("targetPlatform", "MINIKUBE", "Target platform: MINIKUBE or AZURE")

	runCmd.Flags().String("publicRegistryUserName",  "", "Username to download internal images from the public docker registry. Alternatively you may use PUBLIC_REGISTRY_USERNAME")
	runCmd.Flags().String("publicRegistryPassword", "", "Password to download internal images from the public docker registry. Alternatively you may use PUBLIC_REGISTRY_PASSWORD")
	runCmd.Flags().String("publicRegistryURL", "", "URL of the public docker registry. Alternatively you may use PUBLIC_REGISTRY_URL")
	runCmd.Flags().Uint32("ztSidecarPort", 1000, "Port where the ZT sidecar expects route updates")
	runCmd.Flags().String("caCertPath", "", "Path for the CA certificate")
	runCmd.Flags().String("clientCertPath", "", "Path for the client certificate")
	runCmd.Flags().Bool("skipServerCertValidation", true, "Skip CA authentication validation")

	viper.BindPFlags(runCmd.Flags())
}

func Run() {

	config := config.Config{
		Debug:                debugLevel,
		Port:                 uint32(viper.GetInt32("port")),
		MetricsPort:          uint32(viper.GetInt32("metricsPort")),
		ClusterAPIAddress:    viper.GetString("clusterAPIAddress"),
		ManagementHostname:   viper.GetString("managementHostname"),
		ClusterAPIHostname:   viper.GetString("clusterAPIHostname"),
		ClusterAPIPort:       uint32(viper.GetInt32("clusterAPIPort")),
		UseTLSForClusterAPI:  viper.GetBool("useTLSForClusterAPI"),
		LoginHostname:        viper.GetString("loginHostname"),
		LoginPort:            uint32(viper.GetInt32("loginPort")),
		UseTLSForLogin:       viper.GetBool("useTLSForLogin"),
		ClusterPublicHostname:viper.GetString("clusterPublicHostname"),
		DeploymentMgrAddress: viper.GetString("depMgrAddress"),
		Local:                viper.GetBool("local"),
		Email:                viper.GetString("email"),
		Password:             viper.GetString("password"),
		DNS:                  viper.GetString("dns"),
		TargetPlatformName:   viper.GetString("targetPlatform"),
		PublicCredentials:    grpc_application_go.ImageCredentials{
			Username:             viper.GetString("publicRegistryUserName"),
			Password:             viper.GetString("publicRegistryPassword"),
			Email:                "devops@nalej.com",
			DockerRepository:     viper.GetString("publicRegistryURL"),
		},
		ZTSidecarPort: uint32(viper.GetInt32("ztSidecarPort")),
		CACertPath: viper.GetString("caCertPath"),
		ClientCertPath: viper.GetString("clientCertPath"),
		SkipServerCertValidation: viper.GetBool("skipServerCertValidation"),
	}

	log.Info().Msg("launching deployment manager...")

	deploymentMgrService, err := service.NewDeploymentManagerService(&config)
	if err != nil {
		log.Panic().Err(err)
		panic(err.Error())
	}

	deploymentMgrService.Run()
}
