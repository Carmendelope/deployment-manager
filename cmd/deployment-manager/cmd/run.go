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
    "github.com/nalej/deployment-manager/pkg/kubernetes"
)

var runCmd = &cobra.Command{
    Use: "run",
    Short: "Run deployment manager",
    Long: "Run deployment manager service with... and with...",
    Run: func(cmd *cobra.Command, args [] string) {
        Run()
    },
}

func init() {
    // UNIX Time is faster and smaller than most timestamps
    // If you set zerolog.TimeFieldFormat to an empty string,
    // logs will write with UNIX time
    zerolog.TimeFieldFormat = ""

    RootCmd.AddCommand(runCmd)

    runCmd.Flags().Uint32P("port", "c",5002,"port where deployment manager listens to")
    runCmd.Flags().BoolP("local", "l", false, "indicate local k8s instance")
    viper.BindPFlags(runCmd.Flags())
}

func Run() {
    // Local deployment
    var local bool
    // Server port
    var port uint32

    local = viper.GetBool("local")
    port = uint32(viper.GetInt32("port"))



    exec, err := kubernetes.NewKubernetesExecutor(local)
    if err != nil {
        log.Panic().Err(err)
        panic(err.Error())
    }

    // Run the kubernetes controller
    // Run the kubernetes controller
    kontroller := kubernetes.NewKubernetesController(exec.(*kubernetes.KubernetesExecutor))

    // Now let's start the controller
    var stop chan struct{}
    stop = make(chan struct{})
    log.Info().Msg("launching kubernetes controller...")
    go kontroller.Run(1, stop)
    defer close(stop)

    log.Info().Msg("launching deployment manager...")

    deploymentMgrService, err := service.NewDeploymentManagerService(port, &exec)
    if err != nil {
        log.Panic().Err(err)
        panic(err.Error())
    }

    deploymentMgrService.Run()


}
