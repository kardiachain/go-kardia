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
	"crypto/ecdsa"
	"fmt"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"strings"
)

const KARDIA_PROXY = "KARDIA_PROXY"

// Proxy of Kardia's chain to interface with dual's node, responsible for listening to the chain's
// new block and submiting Kardia's transaction.
type KardiaProxy struct {
	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH, KARDIA)
	name   string
	logger log.Logger

	// Kardia's mainchain stuffs.
	kardiaBc     base.BaseBlockChain
	txPool       *tx_pool.TxPool
	chainHeadCh  chan events.ChainHeadEvent // Used to subscribe for new blocks.
	chainHeadSub event.Subscription

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool

	// The external blockchain that this dual node's interacting with.
	externalChain base.BlockChainAdapter

	// TODO(sontranrad@,namdoh@): Hard-coded, need to be cleaned up.
	kaiSmcAddress *common.Address
	smcABI        *abi.ABI
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

func NewKardiaProxy(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, dualEventPool *event_pool.EventPool, smcAddr *common.Address, smcABIStr string) (*KardiaProxy, error) {
	var err error
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	// Create a specific logger for Kardia Proxy.
	logger := log.New()
	logger.AddTag(KARDIA_PROXY)

	processor := &KardiaProxy{
		name:          configs.KAI,
		kardiaBc:      kardiaBc,
		txPool:        txPool,
		dualBc:        dualBc,
		eventPool:     dualEventPool,
		chainHeadCh:   make(chan events.ChainHeadEvent, 5),
		kaiSmcAddress: smcAddr,
		smcABI:        &smcABI,
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

func (p *KardiaProxy)Init(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, dualEventPool *event_pool.EventPool,
	publishedEndpoint, subscribedEndpoint *string) error {
	// Create a specific logger for Kardia Proxy.
	logger := log.New()
	logger.AddTag(KARDIA_PROXY)

	p.logger = logger
	p.name = configs.KAI
	p.kardiaBc = kardiaBc
	p.txPool = txPool
	p.dualBc = dualBc
	p.eventPool = dualEventPool
	p.chainHeadCh = make(chan events.ChainHeadEvent, 5)

	// Start subscription to blockchain head event.
	p.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(p.chainHeadCh)
	return nil
}

// PublishedEndpoint returns publishedEndpoint
func (p *KardiaProxy) PublishedEndpoint() string {
	return ""
}

// SubscribedEndpoint returns subscribedEndpoint
func (p *KardiaProxy) SubscribedEndpoint() string {
	return ""
}

// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
func (p *KardiaProxy) InternalChain() base.BlockChainAdapter {
	return nil
}

func (p *KardiaProxy) ExternalChain() base.BlockChainAdapter {
	return p.externalChain
}

// DualEventPool returns dual's eventPool
func (p *KardiaProxy) DualEventPool() *event_pool.EventPool {
	return p.eventPool
}

// KardiaTxPool returns Kardia Blockchain's tx pool
func (p *KardiaProxy) KardiaTxPool() *tx_pool.TxPool {
	return p.txPool
}

// DualBlockChain returns dual blockchain
func (p *KardiaProxy) DualBlockChain() base.BaseBlockChain {
	return p.dualBc
}

// KardiaBlockChain returns kardia blockchain
func (p *KardiaProxy) KardiaBlockChain() base.BaseBlockChain {
	return p.kardiaBc
}

func (p *KardiaProxy) Logger() log.Logger {
	return p.logger
}

func (p *KardiaProxy) Name() string {
	return p.name
}

func (p *KardiaProxy) SubmitTx(event *types.EventData) error {
	p.logger.Info("Submit to Kardia", "value", event.Data.TxValue, "method", event.Data.TxMethod)
	var err error
	var result interface{}
	switch event.Action.Name {
	case dualnode.CreateKardiaMatchAmountTx:
		p.logger.Info("Handle external event", "source", event.TxSource)
		// These logics temporarily for exchange case , will be dynamic later
		if event.Data.ExtData == nil || len(event.Data.ExtData) < 2 {
			p.logger.Error("Event doesn't contain external data")
			return configs.ErrInsufficientExchangeData
		}
		if event.Data.ExtData[configs.ExchangeV2SourceAddressIndex] == nil || event.Data.ExtData[configs.ExchangeV2DestAddressIndex] == nil {
			p.logger.Error("Missing address in exchange event", "sender", event.Data.ExtData[configs.ExchangeV2SourceAddressIndex],
				"receiver", event.Data.ExtData[configs.ExchangeV2DestAddressIndex])
			return configs.ErrInsufficientExchangeData
		}

		fromType := string(event.Data.ExtData[configs.ExchangeV2SourcePairIndex])
		toType := string(event.Data.ExtData[configs.ExchangeV2DestPairIndex])
		originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])
		srcAddress := string(event.Data.ExtData[configs.ExchangeV2SourceAddressIndex])
		destAddress := string(event.Data.ExtData[configs.ExchangeV2DestAddressIndex])

		p.logger.Info("Create order and match tx:", "source", srcAddress, "dest", destAddress, "txhash", originalTx)

		tx, err := utils.CreateKardiaMatchAmountTx(p.txPool.State(), event.Data.TxValue, srcAddress, destAddress, fromType, toType, originalTx, p.kardiaBc)
		if err != nil {
			p.logger.Error("Fail to create Kardia's tx from DualEvent", "err", err)
			return configs.ErrCreateKardiaTx
		}
		err = p.txPool.AddTx(tx)
		if err != nil {
			p.logger.Error("Fail to add Kardia's tx", "error", err)
			return configs.ErrAddKardiaTx
		}
		p.logger.Info("Submit Kardia's tx successfully", "tx", tx.Hash().String())

		// get smart contract and abi from dual action
		smartContract, kAbi := p.kardiaBc.DB().ReadSmartContractFromDualAction(event.Action.Name)
		if kAbi == nil {
			err := fmt.Errorf("error while getting smc and abi from dual action %v", event.Action.Name)
			p.logger.Error("cannot find abi from action name in submitTx", "err", err)
			return err
		}
		if p.kardiaBc.Config().BaseAccount != nil {
			privateKey := p.kardiaBc.Config().BaseAccount.PrivateKey
			err = p.updateKardiaTxForOrder(common.HexToAddress(smartContract), *kAbi, originalTx, tx.Hash().String(), privateKey)
			if err != nil {
				p.logger.Error("Fail to update Kardia's tx")
			}
		} else {
			p.logger.Error("kardia submitTx, baseAccount not found")
		}
	case dualnode.EnqueueTxPool:
		tx, ok := result.(*types.Transaction)
		if !ok {
			p.logger.Error("type conversion failed")
			return configs.ErrTypeConversionFailed
		}
		err = p.txPool.AddTx(tx)
		if err != nil {
			p.logger.Error("Fail to add Kardia's tx", "error", err)
			return configs.ErrAddKardiaTx
		}
		p.logger.Info("Submit Kardia's tx successfully", "txhash", tx.Hash().String())
	}
	p.logger.Error("Submit to Kardia", "value", event.Data.TxValue, "method", event.Data.TxMethod, "action", event.Action.Name)
	return nil
}

// ComputeTxMetadata precomputes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (p *KardiaProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	// Compute Kardia's tx from the DualEvent.
	// TODO(thientn,namdoh): Remove hard-coded account address here.
	if event.Data.TxMethod == configs.ExternalDepositFunction {
		// These logics temporarily for exchange case , will be dynamic later
		if event.Data.ExtData == nil {
			log.Error("Event doesn't contain external data")
			return nil, configs.ErrFailedGetEventData
		}
		if event.Data.TxValue.Cmp(big.NewInt(0)) == 0 {
			return nil, configs.ErrInsufficientExchangeData
		}
		if event.Data.ExtData[configs.ExchangeV2SourceAddressIndex] == nil || event.Data.ExtData[configs.ExchangeV2DestAddressIndex] == nil {
			log.Error("Missing address in exchange event", "send", event.Data.ExtData[configs.ExchangeV2SourceAddressIndex],
				"receive", event.Data.ExtData[configs.ExchangeV2DestAddressIndex])
			return nil, configs.ErrInsufficientExchangeData
		}
		if event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex] == nil || len(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex]) == 0 {
			log.Error("Missing original tx hash")
		}
		fromType := string(event.Data.ExtData[configs.ExchangeV2SourcePairIndex])
		toType := string(event.Data.ExtData[configs.ExchangeV2DestPairIndex])
		originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])

		log.Info("Computing tx metadata for tx", "hash", originalTx)
		kardiaTx, err := utils.CreateKardiaMatchAmountTx(p.txPool.State(), event.Data.TxValue,
			string(event.Data.ExtData[configs.ExchangeV2SourceAddressIndex]), string(event.Data.ExtData[configs.ExchangeV2DestAddressIndex]),
			fromType, toType, originalTx, p.kardiaBc)
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

func (p *KardiaProxy) Start() {
	// Start event
	go p.loop()
}

func (p *KardiaProxy) RegisterExternalChain(externalChain base.BlockChainAdapter) {
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
		evt, a := p.TxMatchesWatcher(tx)
		if evt != nil && a != nil {
			log.Info("New Kardia's tx detected on smart contract", "addr", tx.To().Hex(), "value", tx.Value())
			p.ExecuteSmcActions(tx, evt, a)
		}
	}
}

// TxMatchesWatcher checks if tx.To matches with watched smart contract, if matched return watched event
func (p *KardiaProxy) TxMatchesWatcher(tx *types.Transaction) (*types.WatcherAction, *abi.ABI) {
	db := p.kardiaBc.DB()
	if tx.To() == nil {
		return nil, nil
	}
	a := db.ReadSmartContractAbi(tx.To().Hex())
	if a != nil {
		// get method and input data from tx
		input := tx.Data()
		method, err := a.MethodById(input)
		if err != nil {
			p.logger.Error("cannot get method from input", "err", err, "address", tx.To().Hex())
			return nil, nil
		}
		// get event from smc address and method
		return db.ReadEvent(tx.To().Hex(), method.Name), a
	}
	return nil, nil
}

func (p *KardiaProxy) ExecuteSmcActions(tx *types.Transaction, watcherAction *types.WatcherAction, abi *abi.ABI) {
	var err error
	p.Logger().Info("ExecuteSmcActions", "action", watcherAction.DualAction)
	switch watcherAction.DualAction {
	// TODO(@kiendn): Note for KSML: execute specific logic then create event which contains dualAction in watcherAction
	case configs.ReleaseEvent:
		err = p.createDualEventFromKaiTxAndEnqueue(tx, &types.DualAction{Name: watcherAction.DualAction}, abi)
	}

	if err != nil {
		log.Error("Error handling tx", "txHash", tx.Hash(), "err", err)
		return
	}
}

// Detects update on kardia master smart contract and creates corresponding dual event to submit to
// dual event pool
func (p *KardiaProxy) createDualEventFromKaiTxAndEnqueue(tx *types.Transaction, action *types.DualAction, abi *abi.ABI) error {
	eventSummary, err := p.extractKardiaTxSummary(tx, abi)
	if err != nil {
		log.Error("Error when extracting Kardia main chain's tx summary.")
		// TODO(#140): Handle smart contract failure correctly.
		panic("Not yet implemented!")
	}
	// TODO(@sontranrad): add dynamic filter if possile, currently we're only interested in matchRequest
	if eventSummary.TxMethod != configs.AddOrderFunction {
		log.Info("Skip tx updating smc for non-matching tx or non-complete-request tx", "method", eventSummary.TxMethod)
		return nil
	}

	log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value",
		eventSummary.TxValue, "hash", tx.Hash())
	nonce := p.eventPool.State().GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	kardiaTxHash := tx.Hash()
	txHash := common.BytesToHash(kardiaTxHash[:])
	dualEvent := types.NewDualEvent(nonce, false /* externalChain */, types.KARDIA, &txHash, &eventSummary, action)
	txMetadata, err := p.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error computing tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetadata
	log.Info("Create DualEvent for Kardia's Tx", "dualEvent", dualEvent)
	err = p.eventPool.AddEvent(dualEvent)
	if err != nil {
		log.Error("Fail to add dual's event", "error", err)
		return err
	}
	log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().String(),
		"eventHash", dualEvent.Hash().String())
	return nil
}

// extractKardiaTxSummary extracts data related to cross-chain exchange to be forwarded to Kardia master smart contract
func (p *KardiaProxy) extractKardiaTxSummary(tx *types.Transaction, abi *abi.ABI) (types.EventSummary, error) {
	// New tx that updates smc, check input method for more filter.
	method, err := abi.MethodById(tx.Data()[0:4])
	if err != nil {
		log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
		return types.EventSummary{}, err
	}
	input := tx.Data()
	var exchangeExternalData [][]byte
	switch method.Name {
	case configs.AddOrderFunction:
		exchangeExternalData = make([][]byte, configs.ExchangeV2NumOfExchangeDataField)
		var decodedInput types.MatchOrderInput
		err = abi.UnpackInput(&decodedInput, configs.AddOrderFunction, input[4:])
		if err != nil {
			log.Error("failed to get external data of exchange contract event", "method", method.Name)
			return types.EventSummary{}, configs.ErrFailedGetEventData
		}
		log.Info("Match request input", "txhash", decodedInput.Txid, "src", decodedInput.FromAddress,
			"dest", decodedInput.Receiver, "srcpair", decodedInput.FromType, "destpair", decodedInput.ToType,
			"amount", decodedInput.Amount, "timestamp", decodedInput.Timestamp)
		exchangeExternalData[configs.ExchangeV2SourceAddressIndex] = []byte(decodedInput.FromAddress)
		exchangeExternalData[configs.ExchangeV2DestAddressIndex] = []byte(decodedInput.Receiver)
		exchangeExternalData[configs.ExchangeV2SourcePairIndex] = []byte(decodedInput.FromType)
		exchangeExternalData[configs.ExchangeV2DestPairIndex] = []byte(decodedInput.ToType)
		exchangeExternalData[configs.ExchangeV2OriginalTxIdIndex] = []byte(decodedInput.Txid)
		exchangeExternalData[configs.ExchangeV2AmountIndex] = decodedInput.Amount.Bytes()
		exchangeExternalData[configs.ExchangeV2TimestampIndex] = decodedInput.Timestamp.Bytes()
	default:
		log.Warn("Unexpected method in extractKardiaTxSummary", "method", method.Name)
	}

	return types.EventSummary{
		TxMethod: method.Name,
		TxValue:  tx.Value(),
		ExtData:  exchangeExternalData,
	}, nil
}

func (p *KardiaProxy) updateKardiaTxForOrder(smartContract common.Address, abi abi.ABI, originalTxId string, kardiaTxId string, privateKey ecdsa.PrivateKey) error {
	state := p.txPool.State()
	tx, err := utils.UpdateKardiaTx(state, smartContract, abi, originalTxId, kardiaTxId,privateKey)
	if err != nil {
		log.Error("Error creating tx updateKardiaTx", "originalTxId", originalTxId, "KardiaTx", kardiaTxId,
			"err", err)
		return err
	}
	err = p.txPool.AddTx(tx)
	if err != nil {
		log.Error("Error updateKardiaTx to txPool", "originalTxId", originalTxId, "KardiaTx", kardiaTxId,
			"err", err)
	}
	log.Info("UpdateKardiaTx for order successfully", "originalTxId", originalTxId, "KardiaTx", kardiaTxId)
	return nil
}

func (p *KardiaProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	panic("this function is not implemented")
}
