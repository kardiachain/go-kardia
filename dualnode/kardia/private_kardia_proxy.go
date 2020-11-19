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

/**
 *  Private Kardia Proxy is used for demonstrate how private chains share information to each other by asking and answer questions.
 */

package kardia

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/kai/tx_pool"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

const PRIVATE_KARDIA = "PRIVATE"

var ErrInsufficientCandidateRequestData = errors.New("insufficient candidate request data")
var ErrInsufficientCandidateResponseData = errors.New("insufficient candidate response data")
var ErrUnpackForwardRequestInfo = errors.New("error unpacking info forward request input")
var ErrUnpackForwardResponseInfo = errors.New("error unpacking info forward response input")
var errAbiNotFound = errors.New("ABI not found")

type KardiaForwardRequestInput struct {
	Email     string
	FromOrgID string
	ToOrgID   string
}

type KardiaForwardResponseInput struct {
	Email     string
	Response  string
	FromOrgID string
	ToOrgID   string
}

// Proxy of Kardia's chain to interface with dual's node, responsible for listening to the chain's
// new block and submiting Kardia's transaction.
type PrivateKardiaProxy struct {

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
	eventPool *event_pool.Pool

	// The external blockchain that this dual node's interacting with.
	externalChain base.BlockChainAdapter

	// TODO(sontranrad@,namdoh@): Hard-coded, need to be cleaned up.
	smcAddress *common.Address
	smcABI     *abi.ABI
}

// PublishedEndpoint returns publishedEndpoint
func (p *PrivateKardiaProxy) PublishedEndpoint() string {
	return ""
}

// SubscribedEndpoint returns subscribedEndpoint
func (p *PrivateKardiaProxy) SubscribedEndpoint() string {
	return ""
}

// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
func (p *PrivateKardiaProxy) InternalChain() base.BlockChainAdapter {
	return nil
}

func (p *PrivateKardiaProxy) ExternalChain() base.BlockChainAdapter {
	return p.externalChain
}

// DualEventPool returns dual's eventPool
func (p *PrivateKardiaProxy) DualEventPool() *event_pool.Pool {
	return p.eventPool
}

// KardiaTxPool returns Kardia Blockchain's tx pool
func (p *PrivateKardiaProxy) KardiaTxPool() *tx_pool.TxPool {
	return p.txPool
}

// DualBlockChain returns dual blockchain
func (p *PrivateKardiaProxy) DualBlockChain() base.BaseBlockChain {
	return p.dualBc
}

// KardiaBlockChain returns kardia blockchain
func (p *PrivateKardiaProxy) KardiaBlockChain() base.BaseBlockChain {
	return p.kardiaBc
}

func (p *PrivateKardiaProxy) Logger() log.Logger {
	return p.logger
}

func (p *PrivateKardiaProxy) Name() string {
	return p.name
}

func NewPrivateKardiaProxy(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, dualEventPool *event_pool.Pool, smcAddr *common.Address, smcABIStr string) (*PrivateKardiaProxy, error) {
	var err error
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}

	// Create a specific logger for Kardia Proxy.
	logger := log.New()
	logger.AddTag(PRIVATE_KARDIA)

	processor := &PrivateKardiaProxy{
		name:        PRIVATE_KARDIA,
		kardiaBc:    kardiaBc,
		txPool:      txPool,
		dualBc:      dualBc,
		eventPool:   dualEventPool,
		chainHeadCh: make(chan events.ChainHeadEvent, 5),
		smcAddress:  smcAddr,
		smcABI:      &smcABI,
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

func (p *PrivateKardiaProxy) Init(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, dualEventPool *event_pool.Pool, publishedEndpoint, subscribedEndpoint *string) error {
	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(PRIVATE_KARDIA)

	if publishedEndpoint == nil || subscribedEndpoint == nil {
		return fmt.Errorf("publishedEndpoint or subscribedEndpoint is empty")
	}
	p.name = PRIVATE_KARDIA
	p.logger = logger
	p.kardiaBc = kardiaBc
	p.txPool = txPool
	p.dualBc = dualBc
	p.eventPool = dualEventPool
	p.chainHeadCh = make(chan events.ChainHeadEvent, 5)
	return nil
}

func (p *PrivateKardiaProxy) SubmitTx(event *types.EventData) error {
	//log.Error("Submit to Kardia", "value", event.Data.TxValue, "method", event.Data.TxMethod)
	//var (
	//	tx  *types.Transaction
	//	err error
	//)
	//switch event.Data.TxMethod {
	//case configs.PrivateChainRequestInfoFunction:
	//	// There is a request comes from external chain, we create a tx to forward it to Kardia candidate exchange smc
	//	tx, err = p.createTxFromExternalRequestData(event)
	//case configs.PrivateChainCompleteRequestFunction:
	//	// There is a response comes from external chain, we create a tx to forward it to Kardia candidate exchange smc
	//	tx, err = p.createTxFromExternalResponseData(event)
	//default:
	//	log.Error("Unsupported method", "method", event.Data.TxMethod)
	//	return configs.ErrUnsupportedMethod
	//}
	//if err != nil {
	//	log.Error("Fail to create Kardia's tx from DualEvent", "err", err)
	//	return configs.ErrCreateKardiaTx
	//}
	//err = p.txPool.AddTx(tx)
	//if err != nil {
	//	log.Error("Fail to add Kardia's tx", "error", err)
	//	return configs.ErrAddKardiaTx
	//}
	//log.Info("Submit Kardia's tx successfully", "txhash", tx.Hash().Hex())
	return nil
}

// ComputeTxMetadata precomputes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (p *PrivateKardiaProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	//var (
	//	tx  *types.Transaction
	//	err error
	//)
	//switch event.Data.TxMethod {
	//case configs.PrivateChainRequestInfoFunction:
	//	tx, err = p.createTxFromExternalRequestData(event)
	//case configs.PrivateChainCompleteRequestFunction:
	//	tx, err = p.createTxFromExternalResponseData(event)
	//default:
	//	return nil, configs.ErrUnsupportedMethod
	//}
	//if err != nil {
	//	return nil, err
	//}
	return &types.TxMetadata{
		TxHash: event.TxHash,
		Target: types.KARDIA,
	}, nil
}

func (p *PrivateKardiaProxy) Start() {
	// Start event
	go p.loop()
}

func (p *PrivateKardiaProxy) RegisterExternalChain(externalChain base.BlockChainAdapter) {
	p.externalChain = externalChain
}

func (p *PrivateKardiaProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	panic("this function is not implemented")
}

func (p *PrivateKardiaProxy) loop() {
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

func (p *PrivateKardiaProxy) handleBlock(block *types.Block) {
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == *p.smcAddress {
			err := p.HandleKardiaTx(tx)
			if err != nil {
				log.Error("Error handling tx", "txHash", tx.Hash(), "err", err)
			}
		}
	}
}

// ExtractKardiaTxSummary extracts information from Kardia tx input about candidate info request / response forwarded
func (p *PrivateKardiaProxy) ExtractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error) {
	method, err := p.smcABI.MethodById(tx.Data()[0:4])
	if err != nil {
		log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
		return types.EventSummary{}, err
	}
	input := tx.Data()
	switch method.Name {
	case configs.KardiaForwardRequestFunction:
		candidateRequestData := make([][]byte, configs.KardiaForwardRequestFields)
		var incomingRequest KardiaForwardRequestInput
		err = p.smcABI.UnpackInput(&incomingRequest, configs.KardiaForwardRequestFunction, input[4:])
		if err != nil {
			log.Error("Error unpack forward request input,", "tx", tx, "err", err)
			return types.EventSummary{}, ErrUnpackForwardRequestInfo
		}
		candidateRequestData[configs.KardiaForwardRequestEmailIndex] = []byte(incomingRequest.Email)
		candidateRequestData[configs.KardiaForwardRequestFromOrgIndex] = []byte(incomingRequest.FromOrgID)
		candidateRequestData[configs.KardiaForwardRequestToOrgIndex] = []byte(incomingRequest.ToOrgID)
		return types.EventSummary{
			TxMethod: configs.KardiaForwardRequestFunction,
			TxValue:  big.NewInt(0),
			ExtData:  candidateRequestData,
		}, nil
	case configs.KardiaForwardResponseFunction:
		forwardedResponseData := make([][]byte, configs.KardiaForwardResponseFields)
		var forwardResponseInput KardiaForwardResponseInput
		err = p.smcABI.UnpackInput(&forwardResponseInput, configs.KardiaForwardResponseFunction, input[4:])
		if err != nil {
			log.Error("Error unpack forward request input,", "tx", tx, "err", err)
			return types.EventSummary{}, ErrUnpackForwardResponseInfo
		}
		forwardedResponseData[configs.KardiaForwardResponseEmailIndex] = []byte(forwardResponseInput.Email)
		forwardedResponseData[configs.KardiaForwardResponseResponseIndex] = []byte(forwardResponseInput.Response)
		forwardedResponseData[configs.KardiaForwardResponseFromOrgIndex] = []byte(forwardResponseInput.FromOrgID)
		forwardedResponseData[configs.KardiaForwardResponseToOrgIndex] = []byte(forwardResponseInput.ToOrgID)
		return types.EventSummary{
			TxMethod: configs.KardiaForwardResponseFunction,
			TxValue:  big.NewInt(0),
			ExtData:  forwardedResponseData,
		}, nil
	default:
		log.Error("Unsupported method", "method", method.Name)
	}
	return types.EventSummary{}, configs.ErrUnsupportedMethod
}

// HandleKardiaTx detects update on kardia candidate exchange smart contract and creates corresponding dual event to submit to
// dual event pool
func (p *PrivateKardiaProxy) HandleKardiaTx(tx *types.Transaction) error {
	//eventSummary, err := p.ExtractKardiaTxSummary(tx)
	//if err != nil {
	//	log.Error("Error when extracting Kardia main chain's tx summary.")
	//	return err
	//}
	//if eventSummary.TxMethod != configs.KardiaForwardResponseFunction && eventSummary.TxMethod != configs.KardiaForwardRequestFunction {
	//	log.Info("Skip tx updating smc not related to candidate exchange", "method", eventSummary.TxMethod)
	//}
	//log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value",
	//	eventSummary.TxValue, "hash", tx.Hash())
	//
	//if p.dualBc.Config().BaseAccount == nil {
	//	return fmt.Errorf("baseAccount is empty")
	//}
	//
	//height := p.dualBc.CurrentBlock().Height()
	//kardiaTxHash := tx.Hash()
	//txHash := common.BytesToHash(kardiaTxHash[:])
	//// TODO(namdoh@): Pass smartcontract actions here.
	//dualEvent := types.NewDualEvent(height, false, types.KARDIA, &txHash, &eventSummary, nil)
	//txMetadata, err := p.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	//if err != nil {
	//	log.Error("method:", "method", eventSummary.TxMethod)
	//	log.Error("Error computing tx metadata", "err", err)
	//	return err
	//}
	//dualEvent.PendingTxMetadata = txMetadata
	//log.Info("Create DualEvent for Kardia's Tx", "dualEvent", dualEvent)
	//if err := p.eventPool.AddEvent(dualEvent); err != nil {
	//	p.Logger().Error("error while adding event", "err", err)
	//	return err
	//}
	//log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().String(),
	//	"eventHash", dualEvent.Hash().String())
	return nil
}

// createTxFromExternalRequestData parses event data to create tx to Kardia candidate exchange smart contract to
// forward a request
func (p *PrivateKardiaProxy) createTxFromExternalRequestData(event *types.EventData) (*types.Transaction, error) {
	//if event.Data.TxMethod != configs.PrivateChainRequestInfoFunction {
	//	return nil, configs.ErrUnsupportedMethod
	//}
	//if event.Data.ExtData == nil || len(event.Data.ExtData) < configs.PrivateChainCandidateRequestFields {
	//	log.Error("Event doesn't contains enough data")
	//	return nil, ErrInsufficientCandidateRequestData
	//}
	//if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestEmailIndex]) {
	//	log.Error("Missing email from external request data")
	//	return nil, ErrInsufficientCandidateRequestData
	//}
	//if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestFromOrgIndex]) {
	//	log.Error("Missing fromOrgId from external request data")
	//	return nil, ErrInsufficientCandidateRequestData
	//}
	//if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestToOrgIndex]) {
	//	log.Error("Missing toOrgId from external request data")
	//	return nil, ErrInsufficientCandidateRequestData
	//}
	//tx, err := utils.CreateForwardRequestTx(string(event.Data.ExtData[configs.PrivateChainCandidateRequestEmailIndex]),
	//	string(event.Data.ExtData[configs.PrivateChainCandidateRequestFromOrgIndex]),
	//	string(event.Data.ExtData[configs.PrivateChainCandidateRequestToOrgIndex]), p.txPool)
	return nil, nil
}

// createTxFromExternalRequestData parses event data to create tx to Kardia candidate exchange smart contract to
// forward a response
func (p *PrivateKardiaProxy) createTxFromExternalResponseData(event *types.EventData) (*types.Transaction, error) {
	//if event.Data.TxMethod != configs.PrivateChainCompleteRequestFunction {
	//	return nil, configs.ErrUnsupportedMethod
	//}
	//if event.Data.ExtData == nil || len(event.Data.ExtData) < configs.PrivateChainCandidateRequestCompletedFields {
	//	log.Error("Event doesn't contains enough data")
	//	return nil, ErrInsufficientCandidateResponseData
	//}
	//if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedEmailIndex]) {
	//	log.Error("Missing email from external response data")
	//	return nil, ErrInsufficientCandidateResponseData
	//}
	//if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedContentIndex]) {
	//	log.Error("Missing content from external response data")
	//	return nil, ErrInsufficientCandidateResponseData
	//}
	//if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedToOrgIDIndex]) {
	//	log.Error("Missing to org ID from external response data")
	//	return nil, ErrInsufficientCandidateResponseData
	//}
	//tx, err := utils.CreateForwardResponseTx(string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedEmailIndex]),
	//	string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedContentIndex]),
	//	string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedFromOrgIDIndex]),
	//	string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedToOrgIDIndex]), p.txPool)
	return nil, nil
}

func (p *PrivateKardiaProxy) Lock() {

}

func (p *PrivateKardiaProxy) UnLock() {

}

// CreateForwardRequestTx creates tx call to Kardia candidate exchange contract to forward a candidate request to another
// external chain
func CreateForwardRequestTx(email string, fromOrgId string, toOrgId string, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	exchangeSmcAbi := configs.GetContractABIByAddress(configs.KardiaCandidateExchangeSmcAddress)
	if exchangeSmcAbi == "" {
		return nil, errAbiNotFound
	}
	kAbi, err := abi.JSON(strings.NewReader(exchangeSmcAbi))
	if err != nil {
		return nil, err
	}
	requestInfoInput, err := kAbi.Pack(configs.KardiaForwardRequestFunction, email, fromOrgId, toOrgId)
	if err != nil {
		return nil, err
	}
	return tx_pool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), common.HexToAddress(configs.KardiaCandidateExchangeSmcAddress), requestInfoInput, txPool, false), nil
}

// CreateForwardResponseTx creates tx call to Kardia candidate exchange contract to fulfill a candidate info request
// from external private chain, receiving private chain will catch the event fired from Kardia exchange contract to process
// candidate info
func CreateForwardResponseTx(email string, response string, fromOrgId string, toOrgId string,
	txPool *tx_pool.TxPool) (*types.Transaction, error) {
	exchangeSmcAbi := configs.GetContractABIByAddress(configs.KardiaCandidateExchangeSmcAddress)
	if exchangeSmcAbi == "" {
		return nil, errAbiNotFound
	}
	kAbi, err := abi.JSON(strings.NewReader(exchangeSmcAbi))
	if err != nil {
		return nil, err
	}
	requestInfoInput, err := kAbi.Pack(configs.KardiaForwardResponseFunction, email, response, fromOrgId, toOrgId)
	if err != nil {
		return nil, err
	}
	return tx_pool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), common.HexToAddress(configs.KardiaCandidateExchangeSmcAddress), requestInfoInput, txPool, false), nil
}

// Return a common private key to call to Kardia smc from dual node
func GetPrivateKeyToCallKardiaSmc() *ecdsa.PrivateKey {
	addrKeyBytes, _ := hex.DecodeString(configs.KardiaPrivKeyToCallSmc)
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	return addrKey
}
