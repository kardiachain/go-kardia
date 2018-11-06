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
	"github.com/kardiachain/go-kardia/lib/crypto"
)

// PublicNodeAPI offers helper utils
type PublicNodeAPI struct {
	node *Node
}

// NewPublicNodeAPI creates a new PublicNodeAPI instance
func NewPublicNodeAPI(node *Node) *PublicNodeAPI {
	return &PublicNodeAPI{node}
}

// PeersCount returns the number of peers that current node can connect to.
func (s *PublicNodeAPI) PeersCount() int {
	return s.node.server.PeerCount()
}

// Peers returns a list of peers
func (s *PublicNodeAPI) Peers() map[string]string {
	peers := s.node.server.Peers()
	results := make(map[string]string)
	for _, peer := range peers {
		results[peer.Name()] = peer.RemoteAddr().String()
	}
	return results
}

// NodeName returns name of current node
func (s *PublicNodeAPI) NodeName() string {
	return s.node.config.Name
}

// NodeInfo returns infomation of current node
func (s *PublicNodeAPI) NodeInfo() map[string]interface{} {
	name := s.node.server.NodeInfo().Name
	address := crypto.PubkeyToAddress(s.node.config.NodeKey().PublicKey).Hex()
	enode := s.node.server.NodeInfo().Enode
	ip := s.node.server.NodeInfo().IP
	id := s.node.server.NodeInfo().ID
	listenAddr := s.node.server.NodeInfo().ListenAddr
	ports := s.node.server.NodeInfo().Ports
	
	return map[string]interface{}{
		"name":			name,
		"address":		address,
		"enode":		enode,
		"id":			id,
		"ip":			ip,
		"listenAddr":		listenAddr,
		"ports":		ports,
	}
}
