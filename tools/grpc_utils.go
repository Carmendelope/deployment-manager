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
// Set of common operations for grpc functions.

package tools

import (
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc/test/bufconn"
    "google.golang.org/grpc"
    "time"
    "context"
    "net"
    "sync"
)

const BufSize = 1024*1024

var (
    // Testing listener to be used in testing environments. It uses a singleton pattern to be instantiated
    // only once if required.
    testListener *bufconn.Listener
    once sync.Once
)



func LaunchServer(server *grpc.Server, listener *bufconn.Listener) {
    go func() {
        if err := server.Serve(listener); err != nil {
            log.Fatal().Errs("failed to listen: %v", []error{err})
        }
    }()
}


func GetConn (listener bufconn.Listener) (*grpc.ClientConn, error){
    ctx := context.Background()
    conn, err := grpc.DialContext(ctx, "bufnet",
        grpc.WithDialer(func(string, time.Duration)(net.Conn, error){
            return listener.Dial()
        }), grpc.WithInsecure())
    if err != nil {
        return nil, err
    }
    return conn, nil
}

func GetDefaultListener() *bufconn.Listener {
    once.Do(func(){
        testListener = bufconn.Listen(BufSize)
    })
    return testListener
}

// GetAvailablePort obtains a free port.
func GetAvailablePort() (int, error) {
    listener, err := net.Listen("tcp", ":0")
    defer listener.Close()
    if err != nil {
        return -1, err
    }
    return listener.Addr().(*net.TCPAddr).Port, nil
}