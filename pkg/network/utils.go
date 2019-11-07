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

package network

// Some common function for networking

import (
	"fmt"

	"github.com/nalej/deployment-manager/pkg/common"
)

// Generate the tuple key and value for a nalej service to be represented.
// params:
//  serv service instance to be processed
// return:
//  variable name, variable value
func GetNetworkingName(serviceName string, organizationId string, appInstanceId string) string {
	value := fmt.Sprintf("%s-%s-%s", common.FormatName(serviceName), organizationId[0:10],
		appInstanceId[0:10])
	return value
}
