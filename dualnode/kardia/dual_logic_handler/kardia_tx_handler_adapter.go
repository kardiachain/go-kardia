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

package dual_logic_handler

import (
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

type KardiaTxHandlerAdapter interface {
	// Parse tx input to corresponding EventData
	ExtractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error)
	// Handle kardia upon logic of each handler
	HandleKardiaTx(tx *types.Transaction, eventPool *event_pool.EventPool, txPool *tx_pool.TxPool) error
	//Computes Tx from the given event, and submit it to the blockchain.
	SubmitTx(event *types.EventData, blockchain base.BaseBlockChain, txPool *tx_pool.TxPool) error
	// Computes Tx from the given event, and returns its metadata or error in case of invalid event data
	ComputeTxMetadata(event *types.EventData, txPool *tx_pool.TxPool) (*types.TxMetadata, error)
	// GetSmcAddress returns contract address that this handler is interested in
	GetSmcAddress() common.Address
	// Init run when proxy is started to send initial txs to prepare data depending on each handler's logic
	Init(pool *tx_pool.TxPool) error
	// Register an external blockchain interface for the handler to interact with
	RegisterExternalChain(externalChain base.BlockChainAdapter)
}
