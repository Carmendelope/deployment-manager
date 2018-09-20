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
    "github.com/rs/zerolog/log"
    "github.com/rs/zerolog"
    "github.com/spf13/viper"
    "path/filepath"
    "strings"
    "os"
)

var RootCmd = &cobra.Command{
    Use: "",
    Short: "Local deployment-manager",
    Long: `The deployment manager is a local instance to control...`,
    //Run: func(cmd *cobra.Command, args []string) {
    //    Run()
    //},
}

// Variables
// Path of the configuration file
var configFile string
// set default values
var debug bool


func Execute() {
    if err := RootCmd.Execute(); err != nil {
        log.Error().Msg(err.Error())
    }
}




func initConfig() {
    // if --config is passed, attempt to parse the config file
    log.Info().Msg("running init config")
    if configFile != "" {

        // get the filepath
        abs, err := filepath.Abs(configFile)
        if err != nil {
            log.Error().AnErr("Error reading filepath: ", err)
        }

        // get the config name
        base := filepath.Base(abs)

        // get the path
        path := filepath.Dir(abs)

        //
        viper.SetConfigName(strings.Split(base, ".")[0])
        viper.AddConfigPath(path)

        viper.AutomaticEnv()

        // Find and read the config file; Handle errors reading the config file
        if err := viper.ReadInConfig(); err != nil {
            log.Fatal().AnErr("Failed to read config file: ", err)
            os.Exit(1)
        }
    } else {
        log.Info().Msg("no config file was set")
    }
}

func init() {
    zerolog.TimeFieldFormat = ""
    cobra.OnInitialize(initConfig)
    // initialization file
    RootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path")
    RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode")

    if debug {
        zerolog.SetGlobalLevel(zerolog.DebugLevel)
    } else {
        zerolog.SetGlobalLevel(zerolog.InfoLevel)
    }
}


/*
// small example for testing
func Run() {
    client,err := kubernetes.NewKubernetesClient(false)

    if err != nil {
        log.Panic().Err(err).Msg("impossible to get k8s client")
        panic(err)
        os.Exit(1)
    }

    client.runDeployment()
    client.Test()
}
*/