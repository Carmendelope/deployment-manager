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
// Generic server using gRPC.

package tools

import (
    "github.com/rs/zerolog/log"
    "net"
    "google.golang.org/grpc"
    "fmt"
)

type GenericGRPCServer struct {
    Port uint32
    Server *grpc.Server
}

// Create a new Conductor server.
//  params:
//   port where the server will be listening
//  return:
//   implementation of a Conductor server
func NewGenericGRPCServer(port uint32) *GenericGRPCServer {
    s := grpc.NewServer()
    return &GenericGRPCServer{port, s}
}

func(c *GenericGRPCServer) Run() {
    log.Info().Msg("Running server...")
    lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", c.Port))

    if err != nil {
        log.Fatal().Errs("failed to listen: %v", []error{err})
    }
    // Register reflection service on gRPC server.
    log.Info().Uint32("port", c.Port).Msg("Launching gRPC server")
    if err := c.Server.Serve(lis); err != nil {
        log.Fatal().Errs("failed to serve: %v", []error{err})
    }

}
