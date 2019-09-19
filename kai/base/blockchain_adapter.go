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

package base

import (
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/lib/log"
)

// An adapter that provide a unified interface for dual node to interact with external (or
// even internal Kardia) blockchains.
type BlockChainAdapter interface {

	// Logger
	Logger() log.Logger

	// Computes Tx from the given event, and submit it to the blockchain.
	SubmitTx(event *types.EventData) error

	// Computes Tx from the given event, and returns its metadata or error in case of invalid event data
	ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error)

	// PublishedEndpoint returns publishedEndpoint
	PublishedEndpoint() string

	// SubscribedEndpoint returns subscribedEndpoint
	SubscribedEndpoint() string

	// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
	InternalChain() BlockChainAdapter

	// ExternalChain returns externalChain which is internal proxy (eg:NeoProxy, TronProxy)
	ExternalChain() BlockChainAdapter

	// DualEventPool returns dual's eventPool
	DualEventPool() *event_pool.Pool

	// DualBlockChain returns dual blockchain
	DualBlockChain() BaseBlockChain

	// KardiaBlockChain returns kardia blockchain
	KardiaBlockChain() BaseBlockChain

	// KardiaTxPool returns Kardia Blockchain's tx pool
	KardiaTxPool() *tx_pool.TxPool

	// Name returns name of proxy (eg: NEO, TRX, ETH, KAI)
	Name() string

	// Register internalchain for current node
	RegisterInternalChain(BlockChainAdapter)

	// Register externalchain for current node
	RegisterExternalChain(adapter BlockChainAdapter)

	// Start proxy
	Start()
}
