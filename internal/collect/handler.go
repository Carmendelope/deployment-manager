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

// Handler for metrics collection

package collect

import (
	"net/http"

	"github.com/nalej/derrors"
)

type Handler struct {
	manager *Manager
	mux     *http.ServeMux
}

func NewHandler(manager *Manager) (*Handler, derrors.Error) {
	handler := &Handler{
		manager: manager,
		mux:     http.NewServeMux(),
	}

	// Register metrics HTTP endpoint
	handler.mux.HandleFunc("/metrics", handler.Metrics)

	return handler, nil
}

// Make this a valid HTTP handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	h.manager.Metrics(w, r)
}
