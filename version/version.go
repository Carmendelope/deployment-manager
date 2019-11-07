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
