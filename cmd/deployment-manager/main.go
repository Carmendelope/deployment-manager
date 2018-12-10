/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package main

import (
    "github.com/nalej/deployment-manager/cmd/deployment-manager/cmd"
    "github.com/nalej/deployment-manager/version"
)

var MainVersion string

var MainCommit string

func main() {
    version.AppVersion = MainVersion
    version.Commit = MainCommit
    cmd.Execute()
}
