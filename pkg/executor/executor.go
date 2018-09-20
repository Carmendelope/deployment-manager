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

package executor

import pbConductor "github.com/nalej/grpc-conductor-go"

// A executor is a middleware that transforms a deployment plan into an executable plan for the given
// platform. These translators are in charge of defining how the platform-specific entities have to be
// created/deployed in order to follow the deployment plan.
type Executor interface {

    // Execute a deployment stage for the current platform.
    //  params:
    //   plan to be executed
    //  return:
    //   error if any
    Execute(plan *pbConductor.DeploymentStage) error

    // Operation to be run in case a stage deployment fails. The rollback should bring the system to
    // the system status before this stage was executed.
    //  params:
    //   plan to be executed
    //   lastDeployed last stage that was deployed
    //  return:
    //   error if any
    StageRollback(plan *pbConductor.DeploymentPlan, lastDeployed int) error
}
