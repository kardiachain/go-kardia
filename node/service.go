/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package node

import (
	"errors"

	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/rpc"
)

var (
	ErrNodeStopped     = errors.New("node not started")
	ErrNodeRunning     = errors.New("node already running")
	ErrServiceUnknown  = errors.New("service unknown")
	ErrNodeStopFailure = errors.New("node failed to stop gracefully")
)

// ServiceContext wraps config data passed from node to all services to be used in service operations.
type ServiceContext struct {
	Config   *NodeConfig
	Services map[string]Service // Map of type name to constructed services
	// EventMux *event.TypeMux           // Event multiplexer
}

// TODO: Database endpoint.

// GetService returns the currently running service for a specific type.
func (ctx *ServiceContext) GetService(typeName string) (Service, error) {
	if running, ok := ctx.Services[typeName]; ok {
		return running, nil
	}
	return nil, ErrServiceUnknown
}

// ServiceConstructor is the function signature of the constructors needed to be
// registered for service instantiation.
type ServiceConstructor func(ctx *ServiceContext) (Service, error)

// Service is an individual protocol that can be registered into a node.
//
// Notes:
//
// • Service life-cycle management is delegated to the node. The service is allowed to
// initialize itself upon creation, but no goroutines should be spun up outside of the
// Start method.
//
// • Restart logic is not required as the node will create a fresh instance
// every time a service is started.
type Service interface {
	// Protocols retrieves the P2P protocols the service wishes to start.
	Protocols() []p2p.Protocol

	// List of all APIs
	APIs() []rpc.API

	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	Start(server *p2p.Server) error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop() error
}
