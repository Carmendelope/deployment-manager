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

package tools

// Part of this code is inspired by https://github.com/processout/grpc-go-pool

import (
    "google.golang.org/grpc"
    "github.com/rs/zerolog/log"
    "sync"
    "errors"
)


// Factory is a function type creating a grpc client
type Factory func(address string) (*grpc.ClientConn, error)

// A map of connections using gRPC.
type ConnectionsMap struct {
    // Map address -> connection
    connections map[string] *grpc.ClientConn
    // mutex for operations
    lock sync.RWMutex
    // Factory to create new connections
    factory Factory
}

func NewConnectionsMap(factory Factory) *ConnectionsMap {
    return &ConnectionsMap{connections: make(map[string] *grpc.ClientConn,0), factory: factory}
}

func (c *ConnectionsMap) GetConnections() []*grpc.ClientConn {
    current_connections := len(c.connections)
    to_return := make([]*grpc.ClientConn,0,current_connections)

    for _,v := range c.connections {
        to_return = append(to_return, v)
    }

    return to_return
}


func(c *ConnectionsMap) GetConnection(address string) (*grpc.ClientConn, error) {
    log.Debug().Str("address",address).Msg("requested connection to map")

    c.lock.Lock()
    defer c.lock.Unlock()

    conn, is_there := c.connections[address]

    if is_there {
        log.Debug().Str("address",address).Msg("found")
        return conn, nil
    }

    log.Debug().Str("address",address).Msg("requested connection was not found")
    return nil, errors.New("connection was not found")
}

func(c *ConnectionsMap) AddConnection(address string) (*grpc.ClientConn, error) {
    c.lock.Lock()
    defer c.lock.Unlock()

    log.Debug().Str("address",address).Msg("add new connection")

    _, is_there := c.connections[address]
    if is_there {
        to_return := errors.New("connection already exists")
        log.Error().Str("address",address).AnErr("connection already existed", to_return)
        return nil, to_return
    }

    conn, err := c.factory(address)
    if err != nil {
        log.Error().Str("address",address).AnErr("error", err)
        return nil,err
    }

    if conn == nil {
        to_return := errors.New("factory generated connection was nil")
        log.Error().Str("address",address).AnErr("factory generated connection was nil", to_return)
        return nil,to_return
    }

    log.Debug().Str("address",address).Msg("connection sucessfully added")
    c.connections[address] = conn
    return conn, nil
}

func (c *ConnectionsMap) RemoveConnection(address string) error {
    c.lock.Lock()
    defer c.lock.Unlock()
    log.Debug().Str("address",address).Msg("requested to be removed")

    // find it
    conn, is_there := c.connections[address]
    if !is_there {
        to_return := errors.New("factory generated connection was nil")
        log.Debug().Str("address",address).AnErr("factory generated connection was nil", to_return)
        return to_return
    }

    err := conn.Close()
    // delete independently of potential errors
    delete(c.connections, address)
    if err != nil {
        log.Error().Str("address",address).AnErr("RemoveConnection", err)
        return err
    }

    log.Debug().Str("address",address).Msg("succesfully removed client")
    return nil

}