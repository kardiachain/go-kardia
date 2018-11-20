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
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// NeoEvent is message sent from NeoPython
type NeoEvent struct {
	BlockNumber *big.Int `json:"blockNumber"  gencodec:"required"`
	TxHash      string   `json:"txHash"       gencodec:"required"`

	// Neo contract hash
	Contract string   `json:"contract"     gencodec:"required"`
	Method   string   `json:"method"       gencodec:"required"`
	Amount   *big.Int `json:"amount"       gencodec:"required"`

	Hash *common.Hash `json:"hash"         rlp:"-"`
}

// NeoApi is used for any rpc request from NeoPython
type NeoApi struct {
	dualBlockchain     *blockchain.DualBlockChain
	internalBlockchain blockchain.BlockChainAdapter
	eventPool          *blockchain.EventPool
}

// NewNeoApi init new NEOApi
func NewNeoApi(dualchain *blockchain.DualBlockChain, internalchain blockchain.BlockChainAdapter, eventPool *blockchain.EventPool) *NeoApi {
	return &NeoApi{dualBlockchain: dualchain,
		internalBlockchain: internalchain,
		eventPool:          eventPool}
}

// NewEvent received data from NeoPython where signedMsg is used for validating the message
func (n *NeoApi) NewEvent(neoEventEncodedBytes string) error {
	byteMsg := common.FromHex(neoEventEncodedBytes)
	var neoEvent NeoEvent
	if err := rlp.DecodeBytes(byteMsg, &neoEvent); err != nil {
		return err
	}
	dualState, err := n.dualBlockchain.State()
	if err != nil {
		log.Error("Fail to get NeoKardia state", "error", err)
		return err
	}
	txHash := common.HexToHash(neoEvent.TxHash)
	nonce := dualState.GetNonce(common.HexToAddress(blockchain.DualStateAddressHex))
	eventSummary := &types.EventSummary{
		TxMethod: neoEvent.Method,
		TxValue:  neoEvent.Amount,
	}
	dualEvent := types.NewDualEvent(nonce, true /* internalChain */, types.NEO, &txHash, eventSummary)
	dualEvent.PendingTxMetadata = n.internalBlockchain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	err = n.eventPool.AddEvent(dualEvent)
	if err != nil {
		log.Info("Failed to add dual event to pool", "err", err)
	}
	return err
}
