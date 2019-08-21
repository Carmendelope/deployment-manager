/*
 * Copyright (C) 2019 Nalej Group - All Rights Reserved
 */

// Handler for metrics collection

package collect

import (
	"net/http"

	"github.com/nalej/derrors"
)

type Handler struct {
	manager *Manager
	mux *http.ServeMux
}

func NewHandler(manager *Manager) (*Handler, derrors.Error) {
	handler := &Handler{
		manager: manager,
		mux: http.NewServeMux(),
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
