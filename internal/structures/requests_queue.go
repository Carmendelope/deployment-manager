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

package structures

import (
	pbDeploymentManager "github.com/nalej/grpc-deployment-manager-go"
	"github.com/phf/go-queue/queue"
	"sync"
)

// Interface for a queue storing deployment requests
type RequestsQueue interface {

	// Obtain next deployment request
	//  returns:
	//   next deployment request, nil if nothing is ready
	NextRequest() *pbDeploymentManager.DeploymentFragmentRequest

	// Check if there are more available requests.
	AvailableRequests() bool

	// Push a request into the queue.
	//  params:
	//   req the requirement to be pushed into.
	//  returns:
	//   error if any
	PushRequest(req *pbDeploymentManager.DeploymentFragmentRequest) error

	// Clear the queue
	Clear()

	// queue length
	Len() int
}

// Basic queue in memory solution.
type MemoryRequestQueue struct {
	// queue for incoming messages
	queue *queue.Queue
	// Mutex for queue operations
	mux sync.RWMutex
}

func NewMemoryRequestQueue() RequestsQueue {
	toReturn := MemoryRequestQueue{queue: queue.New()}
	toReturn.queue.Init()
	return &toReturn
}

// Thread-safe method to access queued requests
func (q *MemoryRequestQueue) NextRequest() *pbDeploymentManager.DeploymentFragmentRequest {
	q.mux.Lock()
	defer q.mux.Unlock()
	toReturn := q.queue.PopFront().(*pbDeploymentManager.DeploymentFragmentRequest)
	return toReturn
}

// Thread-safe function to find whether there are more requests available or not.
func (q *MemoryRequestQueue) AvailableRequests() bool {
	q.mux.RLock()
	defer q.mux.RUnlock()
	available := q.queue.Len() != 0
	return available
}

// Push a new request to the que for later processing.
//  params:
//   req entry to be enqueued
func (q *MemoryRequestQueue) PushRequest(req *pbDeploymentManager.DeploymentFragmentRequest) error {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.queue.PushBack(req)
	return nil
}

func (q *MemoryRequestQueue) Clear() {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.queue.Init()
}

func (q *MemoryRequestQueue) Len() int {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.queue.Len()
}
