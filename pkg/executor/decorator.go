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
 *
 */

package executor

import (
	"github.com/nalej/derrors"
)

// This decorator has to be implemented by any network solution that modifies existing basic or not basic
// k8s deployments and/or extend current deployed entities.

type NetworkDecorator interface {

	// Decoratre with networking solutions entries when they are built.
	Build(aux Deployable, args ...interface{}) derrors.Error

	// Decorate with networking solutions a deployable entry when its deployment is called.
	Deploy(aux Deployable, args ...interface{}) derrors.Error

	// Remove any unnecessary entries when a deployable element is removed.
	Undeploy(aux Deployable, args ...interface{}) derrors.Error
}
