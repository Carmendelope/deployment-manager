/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package cmd

import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/nalej/deployment-manager/pkg/service"
)

var runCmd = &cobra.Command{
    Use: "run",
    Short: "Run deployment manager",
    Long: "Run deployment manager service with... and with...",
    Run: func(cmd *cobra.Command, args [] string) {
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

    runCmd.Flags().Uint32P("port", "p",5200,"port where deployment manager listens to")
    runCmd.Flags().BoolP("local", "l", false, "indicate local k8s instance")
    runCmd.Flags().StringP("conductorAddress","c", "localhost:5000", "conductor address e.g.: 192.168.1.4:5000")
    runCmd.Flags().StringP("networkMgrAddress","n", "localhost:8000", "network address e.g.: 192.168.1.4:8000")
    runCmd.Flags().StringP("depMgrAddress","d", "localhost:5200", "deployment manager address e.g.: deployment-manager.nalej:5200")

    viper.BindPFlags(runCmd.Flags())
}

func Run() {

    config := service.Config{
        Local:            viper.GetBool("local"),
        Port:             uint32(viper.GetInt32("port")),
        ConductorAddress: viper.GetString("conductorAddress"),
        NetworkAddress: viper.GetString("networkMgrAddress"),
        DeploymentMgrAddress: viper.GetString("depMgrAddress"),
    }


    log.Info().Msg("launching deployment manager...")

    deploymentMgrService, err := service.NewDeploymentManagerService(&config)
    if err != nil {
        log.Panic().Err(err)
        panic(err.Error())
    }

    deploymentMgrService.Run()


}
