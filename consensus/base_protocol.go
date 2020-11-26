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

package consensus

import (
	"github.com/kardiachain/go-kardiamain/lib/p2p"
)

type BaseProtocol interface {
	// AddPeer is called by the protocol manager when a new peer is added.
	AddPeer(p p2p.Peer) error

	// RemovePeer is called by the switch when the peer is stopped (due to error
	// or other reason).
	RemovePeer(p *p2p.Peer, reason interface{}) error

	// Broadcast message to all other peers.
	Broadcast(msg interface{}, msgType uint64)
}
