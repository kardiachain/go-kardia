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

package neo

import (
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"
)

const NeoServiceName = "NEO"
const NeoNetworkID = 200

// NeoService implements Service for running neo dual node, including essential APIs
type NeoService struct {
	logger log.Logger // Logger for Neo service

	// Channel for shutting down the service
	shutdownChan chan bool

	dualBlockchain *blockchain.DualBlockChain
	dualEventPool  *blockchain.EventPool
	internalChain  blockchain.BlockChainAdapter

	networkID uint64

	Apis []rpc.API
}

// newNeoService creates a new NeoService object (including the
// initialisation of the NeoService object)
func newNeoService() (*NeoService, error) {
	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(NeoServiceName)

	neoService := &NeoService{
		logger:       logger,
		shutdownChan: make(chan bool),
		networkID:    NeoNetworkID,
	}

	return neoService, nil
}

// Returns a new NeoService
func NewNeoService(ctx *node.ServiceContext) (node.Service, error) {
	neo, err := newNeoService()
	if err != nil {
		return nil, err
	}
	return neo, nil
}

// Initialize sets up blockchains and event pool for NeoService
func (s *NeoService) Initialize(internalBlockchain blockchain.BlockChainAdapter, dualchain *blockchain.DualBlockChain,
	pool *blockchain.EventPool) {
	s.internalChain = internalBlockchain
	s.dualEventPool = pool
	s.dualBlockchain = dualchain
}

func (s *NeoService) NetVersion() uint64 { return s.networkID }

func (s *NeoService) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "neo",
			Version:   "1.0",
			Service:   NewNeoApi(s.dualBlockchain, s.internalChain, s.dualEventPool),
			Public:    true,
		},
	}
}

// Protocols implements Service, returning all the currently configured
// network protocols to start.
func (s *NeoService) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (s *NeoService) SetApis(apis []rpc.API) {
	if len(apis) > 0 {
		for _, api := range apis {
			s.Apis = append(s.Apis, api)
		}
	}
}

func (s *NeoService) Start(server *p2p.Server) error {
	return nil
}

func (s *NeoService) Stop() error {
	return nil
}
