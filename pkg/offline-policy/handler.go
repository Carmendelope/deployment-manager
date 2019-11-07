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

package offline_policy

import (
	"context"
	grpc_common_go "github.com/nalej/grpc-common-go"
)

type Handler struct {
	Manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{Manager: manager}
}

func (h *Handler) RemoveAll(ctx context.Context, req *grpc_common_go.Empty) (*grpc_common_go.Success, error) {
	err := h.Manager.RemoveAll()
	if err != nil {
		return nil, err
	}
	return &grpc_common_go.Success{}, nil
}
