package node

import (
	//"github.com/kardiachain/go-kardia/event"
	"github.com/kardiachain/go-kardia/p2p"
)

// ServiceContext wraps config data passed from node to all services to be used in service operations.
type ServiceContext struct {
	Config   *NodeConfig
	services map[string]Service // Map of type name to constructed services
	// EventMux *event.TypeMux           // Event multiplexer
}

// TODO: Database endpoint.

// GetService returns the currently running service for a specific type.
func (ctx *ServiceContext) GetService(typeName string) (Service, error) {
	if running, ok := ctx.services[typeName]; ok {
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

	// TODO: add RPC endpoints.

	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	Start(server *p2p.Server) error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop() error
}
