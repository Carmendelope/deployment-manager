/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package version

import "fmt"

// AppVersion contains the version of the application. On development, this should be the next version
// to be released. Do not modify this value, use main.MainVersion.
var AppVersion string

// Commit contains the commit identifier that is being built. Do not modify this value, use main.MainCommit.
var Commit string

func GetVersionInfo() string {
	return fmt.Sprintf("version: %s commit: %s\n", AppVersion, Commit)
}