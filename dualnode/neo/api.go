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
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// NeoEvent is message sent from NeoPython
type NeoEvent struct {
	BlockNumber *big.Int `json:"blockNumber"  gencodec:"required"`
	TxHash      string   `json:"txHash"       gencodec:"required"`

	// Neo contract hash
	Contract  string       `json:"contract"     gencodec:"required"`
	Method    string       `json:"method"       gencodec:"required"`
	Sender    string       `json:"sender"       gencodec:"required"`
	Receiver  string       `json:"receiver"     gencodec:"required"`
	Amount    *big.Int     `json:"amount"       gencodec:"required"`
	Timestamp *big.Int     `json:"timestamp"    gencodec:"required"`

	Hash      *common.Hash `json:"hash"         rlp:"-"`
}

// NeoApi is used for any rpc request from NeoPython
type NeoApi struct {
	dualBlockchain     *blockchain.DualBlockChain
	internalBlockchain base.BlockChainAdapter
	eventPool          *event_pool.EventPool
}

// NewNeoApi init new NEOApi
func NewNeoApi(dualchain *blockchain.DualBlockChain, internalchain base.BlockChainAdapter, eventPool *event_pool.EventPool) *NeoApi {
	return &NeoApi{dualBlockchain: dualchain,
		internalBlockchain: internalchain,
		eventPool:          eventPool}
}

// NewEvent received data from NeoPython where signedMsg is used for validating the message
// returns error in case event cannot be added to eventPool
func (n *NeoApi) NewEvent(neoEventEncodedBytes string) error {
	byteMsg := common.FromHex(neoEventEncodedBytes)
	var neoEvent NeoEvent
	err := rlp.DecodeBytes(byteMsg, &neoEvent)
	if err != nil {
		return err
	}
	dualState, err := n.dualBlockchain.State()
	if err != nil {
		log.Error("Fail to get NeoKardia state", "error", err)
		return err
	}
	txHash := common.HexToHash(neoEvent.TxHash)
	nonce := dualState.GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	// Compose extraData struct for fields related to exchange from data extracted by Neo event
	extraData := make([][]byte, configs.ExchangeV2NumOfExchangeDataField)
	extraData[configs.ExchangeV2SourcePairIndex] = []byte(configs.NEO)
	extraData[configs.ExchangeV2DestPairIndex] = []byte(configs.ETH)
	extraData[configs.ExchangeV2SourceAddressIndex] = []byte(neoEvent.Sender)
	extraData[configs.ExchangeV2DestAddressIndex] = []byte(neoEvent.Receiver)
	extraData[configs.ExchangeV2OriginalTxIdIndex] = []byte(neoEvent.TxHash)
	extraData[configs.ExchangeV2AmountIndex] = neoEvent.Amount.Bytes()
	extraData[configs.ExchangeV2TimestampIndex] = neoEvent.Timestamp.Bytes()

	eventSummary := &types.EventSummary{
		TxMethod: neoEvent.Method,
		TxValue:  neoEvent.Amount,
		ExtData:  extraData,
	}
	// TODO(namdoh@): Pass smartcontract actions here.
	actionsTmp := [...]*types.DualAction{
		&types.DualAction{
			Name: dualnode.CreateKardiaMatchAmountTx,
		},
	}
	dualEvent := types.NewDualEvent(nonce, true /* internalChain */, types.NEO, &txHash, eventSummary, &types.DualActions{
		Actions: actionsTmp[:],
	})
	// Compose extraData struct for fields related to exchange
	txMetaData, err := n.internalBlockchain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error compute internal tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetaData
	err = n.eventPool.AddEvent(dualEvent)
	if err != nil {
		log.Error("Failed to add dual event to pool", "err", err)
		return err
	}
	log.Info("Added to dual event pool successfully", "eventHash", dualEvent.Hash().String())
	return nil
}