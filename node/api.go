/*
 *  Copyright 2021 KardiaChain
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
	"fmt"

	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/rpc"
)

// apis returns the collection of built-in RPC APIs.
func (n *Node) apis() []rpc.API {
	return []rpc.API{
		{
			Namespace: "node",
			Version:   "1.0",
			Service:   &publicAdminAPI{n},
		},
	}
}

// PublicAdminAPI is the collection of administrative API methods exposed over
// both secure and unsecure RPC channels.
type publicAdminAPI struct {
	node *Node // Node interfaced by this API
}

// NewPublicAdminAPI creates a new API definition for the public admin methods
// of the node itself.
func NewPublicAdminAPI(node *Node) *publicAdminAPI {
	return &publicAdminAPI{node: node}
}

// Peer return info or a peer
type Peer struct {
	NodeInfo         p2p.DefaultNodeInfo  `json:"node_info"`
	IsOutbound       bool                 `json:"is_outbound"`
	ConnectionStatus p2p.ConnectionStatus `json:"connection_status"`
	RemoteIP         string               `json:"remote_ip"`
}

// Peers retrieves all the information we know about each individual peer at the
// protocol granularity.
func (api *publicAdminAPI) Peers() ([]Peer, error) {
	peersList := api.node.sw.Peers().List()
	peers := make([]Peer, 0, len(peersList))
	for _, peer := range peersList {
		nodeInfo, ok := peer.NodeInfo().(p2p.DefaultNodeInfo)
		if !ok {
			return nil, fmt.Errorf("peer.NodeInfo() is not DefaultNodeInfo")
		}
		peers = append(peers, Peer{
			NodeInfo:         nodeInfo,
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Status(),
			RemoteIP:         peer.RemoteIP().String(),
		})
	}
	return peers, nil
}

// NodeInfo retrieves all the information we know about the host node at the
// protocol granularity.
func (api *publicAdminAPI) NodeInfo() (p2p.NodeInfo, error) {
	nodeInfo := api.node.sw.NodeInfo()
	return nodeInfo, nil
}
