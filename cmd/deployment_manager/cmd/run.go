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

    runCmd.Flags().Uint32P("port", "c",5000,"port where conductor listens to")
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

    log.Info().Msg("launching deployment manager...")

    exec, err := kubernetes.NewKubernetesExecutor(local)
    if err != nil {
        log.Panic().Err(err)
        panic(err.Error())
    }

    deploymentMgrService, err := service.NewDeploymentManagerService(port, &exec)
    if err != nil {
        log.Panic().Err(err)
        panic(err.Error())
    }

    deploymentMgrService.Run()


}
