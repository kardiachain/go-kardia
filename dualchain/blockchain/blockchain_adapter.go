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

package blockchain

import (
	"github.com/kardiachain/go-kardia/types"
)

// An adapter that provide a unified interface for dual node to interact with external (or
// even internal Kardia) blockchains.
type BlockChainAdapter interface {
	// Computes Tx from the given event, and submit it to the blockchain.
	SubmitTx(event *types.EventData) error

	// Computes Tx from the given event, and returns its metadata or error in case of invalid event data
	ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error)
}
