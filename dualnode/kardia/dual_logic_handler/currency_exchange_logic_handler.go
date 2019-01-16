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
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
)

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

type CurrencyExchangeLogicHandler struct {
	smcAddress *common.Address
	smcABI     *abi.ABI
	externalChain base.BlockChainAdapter
}

func NewCurrencyExchangeLogicHandler(smcAddr *common.Address, smcABI *abi.ABI) (*CurrencyExchangeLogicHandler, error) {
	handler := &CurrencyExchangeLogicHandler{smcAddress: smcAddr, smcABI: smcABI}
	return handler, nil
}

// RegisterExternalChain attaches an external blockchain adapter for this handler
func (h *CurrencyExchangeLogicHandler) RegisterExternalChain(externalChain base.BlockChainAdapter) {
	h.externalChain = externalChain
}

func (h *CurrencyExchangeLogicHandler) GetSmcAddress() common.Address {
	return *h.smcAddress
}

// SubmitTx submits a tx to kardia to notify a deposit from external chain
func (h *CurrencyExchangeLogicHandler) SubmitTx(event *types.EventData, blockchain base.BaseBlockChain,
	txPool *tx_pool.TxPool) error {
	log.Error("Submit to Kardia", "value", event.Data.TxValue, "method", event.Data.TxMethod)
	// These logics temporarily for exchange case , will be dynamic later
	if event.Data.ExtData == nil || len(event.Data.ExtData) < 2 {
		log.Error("Event doesn't contain external data")
		return configs.ErrInsufficientExchangeData
	}
	if event.Data.ExtData[configs.ExchangeDataSourceAddressIndex] == nil || event.Data.ExtData[configs.ExchangeDataDestAddressIndex] == nil {
		log.Error("Missing address in exchange event", "sender", event.Data.ExtData[configs.ExchangeDataSourceAddressIndex],
			"receiver", event.Data.ExtData[configs.ExchangeDataDestAddressIndex])
		return configs.ErrInsufficientExchangeData
	}
	statedb, err := blockchain.State()
	if err != nil {
		return err
	}
	sale1, receive1, err1 := utils.CallGetRate(configs.ETH2NEO, blockchain, statedb, h.smcAddress, h.smcABI)
	if err1 != nil {
		return err1
	}
	sale2, receive2, err2 := utils.CallGetRate(configs.NEO2ETH, blockchain, statedb, h.smcAddress, h.smcABI)
	if err2 != nil {
		return err2
	}
	log.Info("Rate before matching", "pair", configs.ETH2NEO, "sale", sale1, "receive", receive1)
	log.Info("Rate before matching", "pair", configs.NEO2ETH, "sale", sale2, "receive", receive2)
	log.Info("Create match tx:", "source", event.Data.ExtData[configs.ExchangeDataSourceAddressIndex],
		"dest", event.Data.ExtData[configs.ExchangeDataDestAddressIndex])
	tx, err := utils.CreateKardiaMatchAmountTx(txPool.State(), event.Data.TxValue,
		string(event.Data.ExtData[configs.ExchangeDataSourceAddressIndex]),
		string(event.Data.ExtData[configs.ExchangeDataDestAddressIndex]), event.TxSource)
	if err != nil {
		log.Error("Fail to create Kardia's tx from DualEvent", "err", err)
		return configs.ErrCreateKardiaTx
	}
	err = txPool.AddLocal(tx)
	if err != nil {
		log.Error("Fail to add Kardia's tx", "error", err)
		return configs.ErrAddKardiaTx
	}
	log.Info("Submit Kardia's tx successfully", "txhash", tx.Hash().String())
	return nil
}

// ComputeTxMetadata computes metadata of tx that will be submitted to Kardia master smart contract
func (h *CurrencyExchangeLogicHandler) ComputeTxMetadata(event *types.EventData, txPool *tx_pool.TxPool) (*types.TxMetadata, error) {
	// Compute Kardia's tx from the DualEvent.
	// TODO(thientn,namdoh): Remove hard-coded account address here.
	if event.Data.TxMethod == configs.ExternalDepositFunction {
		// These logics temporarily for exchange case , will be dynamic later
		if event.Data.ExtData == nil {
			log.Error("Event doesn't contain external data")
			return nil, configs.ErrFailedGetEventData
		}
		if event.Data.ExtData[configs.ExchangeDataDestAddressIndex] == nil || string(event.Data.ExtData[configs.ExchangeDataDestAddressIndex]) == "" ||
			event.Data.TxValue.Cmp(big.NewInt(0)) == 0 {
			return nil, configs.ErrInsufficientExchangeData
		}
		if event.Data.ExtData[configs.ExchangeDataSourceAddressIndex] == nil || event.Data.ExtData[configs.ExchangeDataDestAddressIndex] == nil {
			log.Error("Missing address in exchange event", "send", event.Data.ExtData[configs.ExchangeDataSourceAddressIndex],
				"receive", event.Data.ExtData[configs.ExchangeDataDestAddressIndex])
			return nil, configs.ErrInsufficientExchangeData
		}
		kardiaTx, err := utils.CreateKardiaMatchAmountTx(txPool.State(), event.Data.TxValue,
			string(event.Data.ExtData[configs.ExchangeDataSourceAddressIndex]), string(event.Data.ExtData[configs.ExchangeDataDestAddressIndex]), event.TxSource)
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

// ExtractKardiaTxSummary extracts data related to cross-chain exchange to be forwarded to Kardia master smart contract
func (h *CurrencyExchangeLogicHandler) ExtractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error) {
	// New tx that updates smc, check input method for more filter.
	method, err := h.smcABI.MethodById(tx.Data()[0:4])
	if err != nil {
		log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
		return types.EventSummary{}, err
	}
	input := tx.Data()
	var exchangeExternalData [][]byte
	switch method.Name {
	case configs.MatchFunction:
		exchangeExternalData = make([][]byte, configs.NumOfExchangeDataField)
		var decodedInput MatchRequestInput
		err = h.smcABI.UnpackInput(&decodedInput, configs.MatchFunction, input[4:])
		if err != nil {
			log.Error("failed to get external data of exchange contract event", "method", method.Name)
			return types.EventSummary{}, configs.ErrFailedGetEventData
		}
		log.Info("Match request input", "src", decodedInput.SrcAddress, "dest", decodedInput.DestAddress,
			"srcpair", decodedInput.SrcPair, "destpair", decodedInput.DestPair, "amount", decodedInput.Amount.String())
		exchangeExternalData[configs.ExchangeDataSourceAddressIndex] = []byte(decodedInput.SrcAddress)
		exchangeExternalData[configs.ExchangeDataDestAddressIndex] = []byte(decodedInput.DestAddress)
		exchangeExternalData[configs.ExchangeDataSourcePairIndex] = []byte(decodedInput.SrcPair)
		exchangeExternalData[configs.ExchangeDataDestPairIndex] = []byte(decodedInput.DestPair)
		exchangeExternalData[configs.ExchangeDataAmountIndex] = decodedInput.Amount.Bytes()
	case configs.CompleteFunction:
		exchangeExternalData = make([][]byte, configs.NumOfCompleteRequestDataField)
		var decodedInput CompleteRequestInput
		err = h.smcABI.UnpackInput(&decodedInput, configs.CompleteFunction, input[4:])
		if err != nil {
			log.Error("failed to get external data of exchange contract event", "method", method.Name)
			return types.EventSummary{}, configs.ErrFailedGetEventData
		}
		log.Info("Complete request input", "ID", decodedInput.RequestID, "pair", decodedInput.Pair)
		exchangeExternalData[configs.ExchangeDataCompleteRequestIDIndex] = decodedInput.RequestID.Bytes()
		exchangeExternalData[configs.ExchangeDataCompletePairIndex] = []byte(decodedInput.Pair)
	default:
		log.Warn("Unexpected method in extractKardiaTxSummary", "method", method.Name)
	}

	return types.EventSummary{
		TxMethod: method.Name,
		TxValue:  tx.Value(),
		ExtData:  exchangeExternalData,
	}, nil
}

// HandleKardiaTx detects update on kardia master smart contract and creates corresponding dual event to submit to
// dual event pool
func (h *CurrencyExchangeLogicHandler) HandleKardiaTx(tx *types.Transaction, eventPool *event_pool.EventPool,
	txPool *tx_pool.TxPool) error {
	eventSummary, err := h.ExtractKardiaTxSummary(tx)
	if err != nil {
		log.Error("Error when extracting Kardia main chain's tx summary.")
		// TODO(#140): Handle smart contract failure correctly.
		panic("Not yet implemented!")
	}
	// TODO(@sontranrad): add dynamic filter if possile, currently we're only interested in matchRequest
	// and completeRequest method
	if eventSummary.TxMethod != configs.MatchFunction && eventSummary.TxMethod != configs.CompleteFunction {
		log.Info("Skip tx updating smc for non-matching tx or non-complete-request tx", "method", eventSummary.TxMethod)
	}
	log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value",
		eventSummary.TxValue, "hash", tx.Hash())
	nonce := eventPool.State().GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	kardiaTxHash := tx.Hash()
	txHash := common.BytesToHash(kardiaTxHash[:])
	dualEvent := types.NewDualEvent(nonce, false /* externalChain */, types.KARDIA, &txHash, &eventSummary)
	txMetadata, err := h.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error computing tx metadata", "err", err)
	}
	dualEvent.PendingTxMetadata = txMetadata
	log.Info("Create DualEvent for Kardia's Tx", "dualEvent", dualEvent)
	err = eventPool.AddEvent(dualEvent)
	if err != nil {
		log.Error("Fail to add dual's event", "error", err)
	}
	log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().String(),
		"eventHash", dualEvent.Hash().String())
	return nil
}

// initRate send 2 tx to Kardia exchange smart contract for ETH-NEO and NEO-ETH.
// FIXME: (sontranrad) revisit here to simplify the logic and remove need of updating 2 time if possible
func (h *CurrencyExchangeLogicHandler) InitRate(pool *tx_pool.TxPool) {
	// Set rate for 2 pair
	tx1, err := utils.CreateKardiaSetRateTx(configs.ETH2NEO, big.NewInt(1), big.NewInt(10), pool.State())
	if err != nil {
		log.Error("Failed to create add rate tx", "err", err)
		return
	}
	err = pool.AddLocal(tx1)
	if err != nil {
		log.Error("Failed to add rate tx to pool", "err", err)
		return
	}
	tx2, err := utils.CreateKardiaSetRateTx(configs.NEO2ETH, big.NewInt(10), big.NewInt(1), pool.State())
	if err != nil {
		log.Error("Failed to create add rate tx", "err", err)
		return
	}
	err = pool.AddLocal(tx2)
	if err != nil {
		log.Error("Failed to add rate tx to pool", "err", err)
		return
	}
}

func (h *CurrencyExchangeLogicHandler) Init(pool *tx_pool.TxPool) error {
	h.InitRate(pool)
	return nil
}
