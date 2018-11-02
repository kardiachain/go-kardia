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

package kardia

import (
	"encoding/hex"
	"strings"
	"errors"

	"github.com/kardiachain/go-kardia/dev"
	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	kardiabc "github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/types"
)

var (
	ErrFailedGetState = errors.New("Fail to get Kardia state")
	ErrCreateKardiaTx = errors.New("Fail to create Kardia's Tx from DualEvent")
	ErrAddKardiaTx    = errors.New("Fail to add Tx to Kardia's TxPool")
)

// Representation of Kardia's node when interfacing with dual's chain.
type KardiaProxy struct {
    // Kardia's mainchain stuffs.
	kardiaBc   *kardiabc.BlockChain
	txPool     *kardiabc.TxPool
	chainHeadCh  chan kardiabc.ChainHeadEvent // Used to subscribe for new blocks.
	chainHeadSub event.Subscription

	// Dual blockchain related fields
	dualBc    *dualbc.DualBlockChain
	eventPool *dualbc.EventPool

	// The external blockchain that this dual node's interacting with.
	externalChain dualbc.BlockChainAdapter

    // TODO(namdoh,thientn): Hard-coded for prototyping. This need to be passed dynamically.
	smcAddress *common.Address
	smcABI     *abi.ABI
}

func NewKardiaProxy(kardiaBc *kardiabc.BlockChain, txPool *kardiabc.TxPool, dualBc *dualbc.DualBlockChain, dualEventPool *dualbc.EventPool, smcAddr *common.Address, smcABIStr string) (*KardiaProxy, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &KardiaProxy{
		kardiaBc:   kardiaBc,
		txPool: txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,
		smcAddress: smcAddr,
		smcABI:     &smcABI,

		chainHeadCh: make(chan kardiabc.ChainHeadEvent, 5),
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

func (p *KardiaProxy) SubmitTx(event *types.EventData) error {
	kardiaStateDB, err := p.kardiaBc.State()
	if err != nil {
		log.Error("Fail to get Kardia state", "error", err)
		return ErrFailedGetState
	}

	// TODO(thientn,namdoh): Remove hard-coded genesisAccount here.
	addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[dev.MockKardiaAccountForMatchEthTx])
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	tx := CreateKardiaMatchAmountTx(addrKey, kardiaStateDB, event.Data.TxValue, 1)
	if tx == nil {
		log.Error("Fail to create Kardia's tx from DualEvent")
		return ErrCreateKardiaTx
	}

	if err := p.txPool.AddLocal(tx); err != nil {
		log.Error("Fail to add Kardia's tx", "error", err)
		return ErrAddKardiaTx
	}
	log.Info("Submit Kardia's tx successfully", "txHash", tx.Hash().Fingerprint())

	return nil
}

func (n *KardiaProxy) ComputeTxMetadata(event *types.EventData) *types.TxMetadata {
	// Compute Kardia's tx from the DualEvent.
	// TODO(thientn,namdoh): Remove hard-coded account address here.
	addrKeyBytes, _ := hex.DecodeString(dev.GenesisAddrKeys[dev.MockKardiaAccountForMatchEthTx])
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	kardiaStateDB, err := n.kardiaBc.State()
	if err != nil {
		log.Error("Fail to get Kardia state", "error", err)
		return nil
	}
	// TODO(namdoh@): Pass eventSummary.TxSource to matchType.
	kardiaTx := CreateKardiaMatchAmountTx(addrKey, kardiaStateDB, event.Data.TxValue, 1)
	return &types.TxMetadata{
		TxHash: kardiaTx.Hash(),
		Target: types.KARDIA,
	}
}

func (p *KardiaProxy) Start() {
	// Start event loop
	go p.loop()
}

func (p *KardiaProxy) RegisterExternalChain(externalChain dualbc.BlockChainAdapter) {
	p.externalChain = externalChain
}

func (p *KardiaProxy) loop() {
	if p.externalChain == nil {
	    panic("External chain needs not to be nil.")
	}
	
	for {
		select {
		case ev := <-p.chainHeadCh:
			if ev.Block != nil {
				// New block
				// TODO(thietn): concurrency improvement. Consider call new go routine, or have height atomic counter.
				p.handleBlock(ev.Block)
			}
		case err := <-p.chainHeadSub.Err():
			log.Error("Error while listening to new blocks", "error", err)
			return
		}
	}
}

func (p *KardiaProxy) handleBlock(block *types.Block) {
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == *p.smcAddress {
			eventSummary, err := p.extractKardiaTxSummary(tx)
			if err != nil {
				log.Error("Error when extracting Kardia main chain's tx summary.")
				// TODO(#140): Handle smart contract failure correctly.
				panic("Not yet implemented!")
			}
			log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value", eventSummary.TxValue, "hash", tx.Hash().Fingerprint())

            // TODO(namdoh,thientn): This is Eth's specific stuff that needs to be removed asap.
			// New tx that updates smc, check input method for more filter.
			if eventSummary.TxMethod == "removeEth" {
				// Not set flag here. If the block contains only the removeEth, skip look up the amount to avoid infinite loop.
				log.Info("Skip tx updating smc to remove Eth", "method", eventSummary.TxMethod)
				continue
			}

			dualStateDB, err := p.dualBc.State()
			if err != nil {
				log.Error("Fail to get Kardia state", "error", err)
				return
			}
			nonce := dualStateDB.GetNonce(common.HexToAddress(dualbc.DualStateAddressHex))
			kardiaTxHash := tx.Hash()
			txHash := common.BytesToHash(kardiaTxHash[:])
			dualEvent := types.NewDualEvent(nonce, false /* externalChain */, types.KARDIA, &txHash, &eventSummary)
			dualEvent.PendingTxMetadata = p.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)

			log.Info("Create DualEvent for Kardia's Tx", "dualEvent", dualEvent)
			if err := p.eventPool.AddEvent(dualEvent); err != nil {
				log.Error("Fail to add dual's event", "error", err)
				return
			}
			log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().Fingerprint(), "eventHash", dualEvent.Hash().Fingerprint())
		}
	}
}

func (p *KardiaProxy) extractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error) {
	// New tx that updates smc, check input method for more filter.
	method, err := p.smcABI.MethodById(tx.Data()[0:4])
	if err != nil {
		log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
		return types.EventSummary{}, err
	}

	return types.EventSummary{
		TxMethod: method.Name,
		TxValue:  tx.Value(),
	}, nil
}
