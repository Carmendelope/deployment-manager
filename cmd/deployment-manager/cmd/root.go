/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
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
    "github.com/nalej/golang-template/version"
)

var RootCmd = &cobra.Command{
    Use: "",
    Short: "Local deployment-manager",
    Long: `The deployment manager is a local instance to control...`,
    Version: "unknown-version",
    //Run: func(cmd *cobra.Command, args []string) {
    //    Run()
    //},
}

// Variables
// Path of the configuration file
var configFile string
// set default values
var debugLevel bool
// set console logging format
var consoleLogging bool



func Execute() {
    SetupLogging()
    RootCmd.SetVersionTemplate(version.GetVersionInfo())
    if err := RootCmd.Execute(); err != nil {
        log.Error().Msg(err.Error())
    }
}




func initConfig() {
    // if --config is passed, attempt to parse the config file
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
    }
}

func init() {
    cobra.OnInitialize(initConfig)
    // initialization file
    RootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path")
    RootCmd.PersistentFlags().BoolVar(&debugLevel, "debug", false, "enable debugLevel mode")
    RootCmd.PersistentFlags().BoolVar(&consoleLogging, "consoleLogging", false, "Pretty print logging")


}


// SetupLogging sets the debugLevel level and console logging if required.
func SetupLogging() {
    zerolog.TimeFieldFormat = ""
    zerolog.SetGlobalLevel(zerolog.InfoLevel)
    if debugLevel {
        zerolog.SetGlobalLevel(zerolog.DebugLevel)
    }

    if consoleLogging {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }
}
