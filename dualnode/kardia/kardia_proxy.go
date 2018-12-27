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
	"errors"
	"strings"

	dualbc "github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	kardiabc "github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
)

var (
	ErrFailedGetState           = errors.New("Fail to get Kardia state")
	ErrCreateKardiaTx           = errors.New("Fail to create Kardia's Tx from DualEvent")
	ErrAddKardiaTx              = errors.New("Fail to add Tx to Kardia's TxPool")
	ErrInsufficientExchangeData = errors.New("Insufficient exchange external data")
	ErrFailedGetEventData       = errors.New("Fail to get event external data")
	ErrNoMatchedRequest         = errors.New("Request has no matched opponent")
	ErrUnsupportedMethod        = errors.New("Method is not supported by dual logic")
)

// Proxy of Kardia's chain to interface with dual's node, responsible for listening to the chain's
// new block and submiting Kardia's transaction .
const MatchFunction = "matchRequest"
const CompleteFunction = "completeRequest"
const ExternalDepositFunction = "deposit"

type KardiaProxy struct {
	// Kardia's mainchain stuffs.
	kardiaBc     *kardiabc.BlockChain
	txPool       *kardiabc.TxPool
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

type MatchRequestInput struct {
	SrcPair     string
	DestPair    string
	SrcAddress  string
	DestAddress string
	Amount      *big.Int
}

type CompleteRequestInput struct {
	// RequestID is ID of request stored in Kardia exchange smc
	RequestID *big.Int
	// Pair is original direction of competed request, for ETH-NEO request, pair is "ETH-NEO"
	Pair string
}

type MatchedRequest struct {
	MatchedRequestID *big.Int `abi:"matchedRequestID"`
	DestAddress      string   `abi:"destAddress"`
	SendAmount       *big.Int `abi:"sendAmount"`
}

func NewKardiaProxy(kardiaBc *kardiabc.BlockChain, txPool *kardiabc.TxPool, dualBc *dualbc.DualBlockChain, dualEventPool *dualbc.EventPool, smcAddr *common.Address, smcABIStr string) (*KardiaProxy, error) {
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	processor := &KardiaProxy{
		kardiaBc:   kardiaBc,
		txPool:     txPool,
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
	// We currently only handle external deposit from outside
	if event.Data.TxMethod != ExternalDepositFunction {
		return ErrUnsupportedMethod
	}
	log.Error("Submit to Kardia", "value", event.Data.TxValue, "method", event.Data.TxMethod)
	// These logics temporarily for exchange case , will be dynamic later
	if event.Data.ExtData == nil || len(event.Data.ExtData) < 2 {
		log.Error("Event doesn't contain external data")
		return ErrInsufficientExchangeData
	}
	if event.Data.ExtData[0] == nil || event.Data.ExtData[1] == nil {
		log.Error("Missing address in exchange event", "sender", event.Data.ExtData[0],
			"receiver", event.Data.ExtData[1])
		return ErrInsufficientExchangeData
	}
	stateDb, err := p.kardiaBc.State()
	if err != nil {
		log.Error("Error getting state", "err", err)
		return ErrFailedGetState
	}
	sale1, receive1, err1 := CallGetRate(ETH2NEO, p.kardiaBc, stateDb)
	if err1 != nil {
		return err1
	}
	sale2, receive2, err2 := CallGetRate(NEO2ETH, p.kardiaBc, stateDb)
	if err2 != nil {
		return err2
	}
	log.Info("Rate before matching", "pair", ETH2NEO, "sale", sale1, "receive", receive1)
	log.Info("Rate before matching", "pair", NEO2ETH, "sale", sale2, "receive", receive2)
	log.Info("Create match tx:", "source", event.Data.ExtData[ExchangeDataSourceAddressIndex],
		"dest", event.Data.ExtData[ExchangeDataDestAddressIndex])
	tx, err := CreateKardiaMatchAmountTx(p.txPool.State(), event.Data.TxValue,
		string(event.Data.ExtData[ExchangeDataSourceAddressIndex]), string(event.Data.ExtData[ExchangeDataDestAddressIndex]), event.TxSource)
	if err != nil {
		log.Error("Fail to create Kardia's tx from DualEvent", "err", err)
		return ErrCreateKardiaTx
	}
	err = p.txPool.AddLocal(tx)
	if err != nil {
		log.Error("Fail to add Kardia's tx", "error", err)
		return ErrAddKardiaTx
	}
	log.Info("Submit Kardia's tx successfully", "txhash", tx.Hash().String())
	return nil

}

// ComputeTxMetadata precomputes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (n *KardiaProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	// Compute Kardia's tx from the DualEvent.
	// TODO(thientn,namdoh): Remove hard-coded account address here.
	if event.Data.TxMethod == ExternalDepositFunction {
		// These logics temporarily for exchange case , will be dynamic later
		if event.Data.ExtData == nil {
			log.Error("Event doesn't contain external data")
			return nil, ErrFailedGetEventData
		}
		if event.Data.ExtData[ExchangeDataDestAddressIndex] == nil || string(event.Data.ExtData[ExchangeDataDestAddressIndex]) == "" ||
			event.Data.TxValue.Cmp(big.NewInt(0)) == 0 {
			return nil, ErrInsufficientExchangeData
		}
		if event.Data.ExtData[ExchangeDataSourceAddressIndex] == nil || event.Data.ExtData[ExchangeDataDestAddressIndex] == nil {
			log.Error("Missing address in exchange event", "send", event.Data.ExtData[ExchangeDataSourceAddressIndex],
				"receive", event.Data.ExtData[ExchangeDataDestAddressIndex])
			return nil, ErrInsufficientExchangeData
		}
		kardiaTx, err := CreateKardiaMatchAmountTx(n.txPool.State(), event.Data.TxValue,
			string(event.Data.ExtData[ExchangeDataSourceAddressIndex]), string(event.Data.ExtData[ExchangeDataDestAddressIndex]), event.TxSource)
		if err != nil {
			return nil, err
		}
		return &types.TxMetadata{
			TxHash: kardiaTx.Hash(),
			Target: types.KARDIA,
		}, nil

	}
	// Temporarily return simple MetaData for other events
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.KARDIA,
	}, nil
}

func (p *KardiaProxy) Start(initRate bool) {
	// Start event
	go p.loop()
	if initRate {
		go p.initRate()
	}
}

// initRate send 2 tx to Kardia exchange smart contract for ETH-NEO and NEO-ETH.
// FIXME: (sontranrad) revisit here to simplify the logic and remove need of updating 2 time if possible
func (p *KardiaProxy) initRate() {
	// Set rate for 2 pair
	tx1, err := CreateKardiaSetRateTx(ETH2NEO, big.NewInt(1), big.NewInt(10), p.txPool.State());
	if err != nil {
		log.Error("Failed to create add rate tx", "err", err)
		return
	}
	err = p.txPool.AddLocal(tx1)
	if err != nil {
		log.Error("Failed to add rate tx to pool", "err", err)
		return
	}
	tx2, err := CreateKardiaSetRateTx(NEO2ETH, big.NewInt(10), big.NewInt(1), p.txPool.State())
	if err != nil {
		log.Error("Failed to create add rate tx", "err", err)
		return
	}
	err = p.txPool.AddLocal(tx2)
	if err != nil {
		log.Error("Failed to add rate tx to pool", "err", err)
		return
	}
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
			// TODO(@sontranrad): add dynamic filter if possile, currently we're only interested in matchRequest
			// and completeRequest method
			if eventSummary.TxMethod != MatchFunction && eventSummary.TxMethod != CompleteFunction {
				log.Info("Skip tx updating smc for non-matching tx or non-complete-request tx", "method", eventSummary.TxMethod)
				continue
			}
			log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value",
				eventSummary.TxValue, "hash", tx.Hash())
			nonce := p.eventPool.State().GetNonce(common.HexToAddress(dualbc.DualStateAddressHex))
			kardiaTxHash := tx.Hash()
			txHash := common.BytesToHash(kardiaTxHash[:])
			dualEvent := types.NewDualEvent(nonce, false /* externalChain */, types.KARDIA, &txHash, &eventSummary)
			txMetadata, err := p.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
			if err != nil {
				log.Error("Error computing tx metadata", "err", err)
				continue
			}
			dualEvent.PendingTxMetadata = txMetadata
			log.Info("Create DualEvent for Kardia's Tx", "dualEvent", dualEvent)
			err = p.eventPool.AddEvent(dualEvent)
			if err != nil {
				log.Error("Fail to add dual's event", "error", err)
				continue
			}
			log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().String(),
				"eventHash", dualEvent.Hash().String())
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
	input := tx.Data()
	var exchangeExternalData [][]byte
	switch method.Name {
	case MatchFunction:
		exchangeExternalData = make([][]byte, NumOfExchangeDataField)
		var decodedInput MatchRequestInput
		err = p.smcABI.UnpackInput(&decodedInput, MatchFunction, input[4:])
		if err != nil {
			log.Error("failed to get external data of exchange contract event", "method", method.Name)
			return types.EventSummary{}, ErrFailedGetEventData
		}
		log.Info("Match request input", "src", decodedInput.SrcAddress, "dest", decodedInput.DestAddress,
			"srcpair", decodedInput.SrcPair, "destpair", decodedInput.DestPair, "amount", decodedInput.Amount.String())
		exchangeExternalData[ExchangeDataSourceAddressIndex] = []byte(decodedInput.SrcAddress)
		exchangeExternalData[ExchangeDataDestAddressIndex] = []byte(decodedInput.DestAddress)
		exchangeExternalData[ExchangeDataSourcePairIndex] = []byte(decodedInput.SrcPair)
		exchangeExternalData[ExchangeDataDestPairIndex] = []byte(decodedInput.DestPair)
		exchangeExternalData[ExchangeDataAmountIndex] = decodedInput.Amount.Bytes()
	case CompleteFunction:
		exchangeExternalData = make([][]byte, NumOfCompleteRequestDataField)
		var decodedInput CompleteRequestInput
		err = p.smcABI.UnpackInput(&decodedInput, CompleteFunction, input[4:])
		if err != nil {
			log.Error("failed to get external data of exchange contract event", "method", method.Name)
			return types.EventSummary{}, ErrFailedGetEventData
		}
		log.Info("Complete request input", "ID", decodedInput.RequestID, "pair", decodedInput.Pair)
		exchangeExternalData[ExchangeDataCompleteRequestIDIndex] = decodedInput.RequestID.Bytes()
		exchangeExternalData[ExchangeDataCompletePairIndex] = []byte(decodedInput.Pair)
	default:
		log.Warn("Unexpected method in extractKardiaTxSummary", "method", method.Name)
	}

	return types.EventSummary{
		TxMethod: method.Name,
		TxValue:  tx.Value(),
		ExtData:  exchangeExternalData,
	}, nil
}
