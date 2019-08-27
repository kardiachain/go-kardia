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

package permissioned

import (
	"fmt"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain"
	"github.com/kardiachain/go-kardia/mainchain/permissioned"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
	"github.com/pkg/errors"
	"math/big"
	"strconv"
	"strings"
)

const SERVICE_NAME = "PRIVATE_DUAL"

var errUnpackingEvent = errors.New("Error unpacking event")
var errInsufficientForwardRequestData = errors.New("insufficient data for forward request")
var errInsufficientForwardResponseData = errors.New("insufficient data for forward response")
var errInvalidTargetChainId = errors.New("The request is not targeted to this private chain")

type ExternalCandidateInfoRequestInput struct {
	Email     string
	FromOrgId string
	ToOrgId   string
}

type RequestCompletedEvent struct {
	Email   string
	Name    string
	Age     uint8
	Addr    common.Address
	Source  string
	ToOrgId string
}

type CompleteRequestInput struct {
	RequestID *big.Int
	Email     string
	Content   string
	ToOrgId   string
}

// PermissionedProxy provides interfaces for PrivateChain Dual node
type PermissionedProxy struct {

	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH, KARDIA)
	name string

	permissionBc base.BaseBlockChain
	txPool       *tx_pool.TxPool
	// TODO: uncomment these lines when we have specific watched smartcontract
	smcAddress *common.Address
	smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool // Event pool of DUAL service.

	privateService *kai.KardiaService

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// Chain head subscription for privatechain new blocks.
	chainHeadCh  chan events.ChainHeadEvent
	chainHeadSub event.Subscription

	privateChainID   *uint64
	logger           log.Logger
	candidateSmcUtil *permissioned.CandidateSmcUtil
}

// NewPermissionedProxy initiates a new private proxy
func NewPermissionedProxy(config *Config, internalBlockchain base.BaseBlockChain,
	txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, eventPool *event_pool.EventPool,
	address *common.Address, smcABIStr string) (*PermissionedProxy, error) {

	logger := log.New()
	logger.AddTag(SERVICE_NAME)

	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}
	// Setup nodeConfig for privatechain
	nodeConfig, err := SetUp(config)
	if err != nil {
		return nil, err
	}

	// New node based on nodeConfig
	n, err := node.NewNode(nodeConfig)
	if err != nil {
		return nil, err
	}

	n.RegisterService(kai.NewKardiaService)

	// Start node
	if err := n.Start(); err != nil {
		return nil, err
	}

	// Get privateService
	var kardiaService *kai.KardiaService
	if err := n.Service(&kardiaService); err != nil {
		return nil, fmt.Errorf("cannot get privateService: %v", err)
	}

	for i := 0; i < nodeConfig.MainChainConfig.EnvConfig.GetNodeSize(); i++ {
		peerURL := nodeConfig.MainChainConfig.EnvConfig.GetNodeMetadata(i).NodeID()
		logger.Info("Adding static peer", "peerURL", peerURL)
		if err := n.AddPeer(peerURL); err != nil {
			return nil, err
		}
	}
	candidateSmcUtil, err := permissioned.NewCandidateSmcUtil(internalBlockchain, utils.GetPrivateKeyToCallKardiaSmc())
	if err != nil {
		logger.Info("Cannot create candidate smc util", "err", err)
		return nil, err
	}
	processor := &PermissionedProxy{
		name:             SERVICE_NAME,
		permissionBc:     internalBlockchain,
		dualBc:           dualBc,
		eventPool:        eventPool,
		txPool:           txPool,
		privateService:   kardiaService,
		chainHeadCh:      make(chan events.ChainHeadEvent, 5),
		logger:           logger,
		smcAddress:       address,
		smcABI:           &smcABI,
		privateChainID:   config.ChainID,
		candidateSmcUtil: candidateSmcUtil,
	}

	processor.chainHeadSub = kardiaService.BlockChain().SubscribeChainHeadEvent(processor.chainHeadCh)
	return processor, nil
}

// TODO(kiendn): permissionedProxy is special case, will implement this function later or separate this case to another code.
func (p *PermissionedProxy) Init(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, dualEventPool *event_pool.EventPool, publishedEndpoint, subscribedEndpoint *string) error {
	panic("this function has not been implemented yet")
}

// PublishedEndpoint returns publishedEndpoint
func (p *PermissionedProxy) PublishedEndpoint() string {
	return ""
}

// SubscribedEndpoint returns subscribedEndpoint
func (p *PermissionedProxy) SubscribedEndpoint() string {
	return ""
}

// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
func (p *PermissionedProxy) InternalChain() base.BlockChainAdapter {
	return p.internalChain
}

func (p *PermissionedProxy) ExternalChain() base.BlockChainAdapter {
	return nil
}

// DualEventPool returns dual's eventPool
func (p *PermissionedProxy) DualEventPool() *event_pool.EventPool {
	return p.eventPool
}

// KardiaTxPool returns Kardia Blockchain's tx pool
func (p *PermissionedProxy) KardiaTxPool() *tx_pool.TxPool {
	return p.txPool
}

// DualBlockChain returns dual blockchain
func (p *PermissionedProxy) DualBlockChain() base.BaseBlockChain {
	return p.dualBc
}

// KardiaBlockChain returns kardia blockchain
func (p *PermissionedProxy) KardiaBlockChain() base.BaseBlockChain {
	return nil
}

func (p *PermissionedProxy) Logger() log.Logger {
	return p.logger
}

func (p *PermissionedProxy) Name() string {
	return p.name
}

func (p *PermissionedProxy) Start() {
	// Start event
	go p.loop()
}

func (p *PermissionedProxy) loop() {
	for {
		select {
		case privateEvent := <-p.chainHeadCh:
			if privateEvent.Block != nil {
				p.handlePrivateBlock(privateEvent.Block)
			}
		case err := <-p.chainHeadSub.Err():
			p.logger.Error("Error while listening to new blocks from privatechain", "error", err)
			panic("error listening")
			return
		}
	}
}

// handleBlock handles privatechain coming blocks and processes watched smc
func (p *PermissionedProxy) handlePrivateBlock(block *types.Block) {
	p.logger.Info("Received block from privatechain", "newBlockHeight", block.Height(), "currentBlock", p.privateService.BlockChain().CurrentBlock().Height())
	for _, tx := range block.Transactions() {
		method, err := p.smcABI.MethodById(tx.Data()[0:4])
		if err != nil {
			p.logger.Error("Error unpacking method", "tx", tx.Hash(), "err", err)
			// TODO(@sontranrad): add a counter to track how many errors we encountered
			continue
		}
		// We process candidate info requests and their responses from private chain to forward to smart contract on Kardia
		if method.Name != configs.PrivateChainRequestInfoFunction && method.Name != configs.PrivateChainCompleteRequestFunction {
			p.logger.Error("Unsupported method", "method", method.Name)
			continue
		}
		eventSummary, err := p.extractPrivateChainTxSummary(tx.Data(), method.Name)
		if err != nil {
			continue
		}
		dualStateDB, err := p.dualBc.State()
		if err != nil {
			p.logger.Error("Fail to get dual state", "error", err)
			return
		}
		nonce := dualStateDB.GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
		privateChainTxHash := tx.Hash()
		txHash := common.BytesToHash(privateChainTxHash[:])
		// Compose dual event and tx metadata from emitted event from private chain smart contract
		// TODO(namdoh@): Pass smartcontract actions here.
		dualEvent := types.NewDualEvent(nonce, true, /* externalChain */
			types.BlockchainSymbol(string(*p.privateChainID)), &txHash, &eventSummary, nil)
		txMetaData, err := p.internalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
		if err != nil {
			p.logger.Error("Error compute internal tx metadata", "err", err)
			continue
		}
		dualEvent.PendingTxMetadata = txMetaData
		p.logger.Info("Create DualEvent for private chain's Tx", "dualEvent", dualEvent)
		err = p.eventPool.AddEvent(dualEvent)
		if err != nil {
			p.logger.Error("Fail to add dual's event", "error", err)
		}
		p.logger.Info("Submitted Private chain 's DualEvent to event pool successfully", "eventHash", dualEvent.Hash().Hex())
	}
}

// extractPrivateChainTxSummary parses event from private chain tx log to EventSummary,
// currently only support 2 methods of private chain candidate smart contract
func (p *PermissionedProxy) extractPrivateChainTxSummary(input []byte, method string) (types.EventSummary, error) {
	switch method {
	case configs.PrivateChainRequestInfoFunction:
		extraData := make([][]byte, configs.PrivateChainCandidateRequestFields)
		var candidateInfoRequestInput ExternalCandidateInfoRequestInput
		unpackErr := p.smcABI.UnpackInput(&candidateInfoRequestInput,
			configs.PrivateChainRequestInfoFunction, input[4:])
		if unpackErr != nil {
			p.logger.Error("Error unpacking event", "err", unpackErr)
			return types.EventSummary{}, errUnpackingEvent
		}
		extraData[configs.PrivateChainCandidateRequestEmailIndex] = []byte(candidateInfoRequestInput.Email)
		extraData[configs.PrivateChainCandidateRequestFromOrgIndex] = []byte(candidateInfoRequestInput.FromOrgId)
		extraData[configs.PrivateChainCandidateRequestToOrgIndex] = []byte(candidateInfoRequestInput.ToOrgId)
		return types.EventSummary{
			TxMethod: configs.PrivateChainRequestInfoFunction,
			TxValue:  big.NewInt(0),
			ExtData:  extraData,
		}, nil
	case configs.PrivateChainCompleteRequestFunction:
		extraData := make([][]byte, configs.PrivateChainCandidateRequestCompletedFields)
		var completeRequestInput CompleteRequestInput
		unpackErr := p.smcABI.UnpackInput(&completeRequestInput,
			configs.PrivateChainCompleteRequestFunction, input[4:])
		if unpackErr != nil {
			p.logger.Error("Error unpacking event", "data", input, "err", unpackErr)
			return types.EventSummary{}, errUnpackingEvent
		}
		extraData[configs.PrivateChainCandidateRequestCompletedFromOrgIDIndex] = []byte(strconv.FormatUint(*p.privateChainID, 10))
		extraData[configs.PrivateChainCandidateRequestCompletedToOrgIDIndex] = []byte(completeRequestInput.ToOrgId)
		extraData[configs.PrivateChainCandidateRequestCompletedEmailIndex] = []byte(completeRequestInput.Email)
		extraData[configs.PrivateChainCandidateRequestCompletedContentIndex] = []byte(completeRequestInput.Content)
		return types.EventSummary{
			TxMethod: configs.PrivateChainCompleteRequestFunction,
			TxValue:  big.NewInt(0),
			ExtData:  extraData,
		}, nil
	default:
		log.Error("Unexpected method for private candidate smc", "method", method)
	}
	return types.EventSummary{}, configs.ErrUnsupportedMethod
}

func (p *PermissionedProxy) SubmitTx(event *types.EventData) error {
	var (
		tx  *types.Transaction
		err error
	)
	switch event.Data.TxMethod {
	case configs.KardiaForwardRequestFunction:
		tx, err = p.createTxFromKardiaForwardedRequest(event, p.privateService.TxPool())
	case configs.KardiaForwardResponseFunction:
		tx, err = p.createTxFromKardiaForwardedResponse(event, p.privateService.TxPool())
	default:
		return configs.ErrUnsupportedMethod
	}
	if err != nil {
		log.Error("Fail to create Kardia's tx from DualEvent", "err", err)
		return configs.ErrCreateKardiaTx
	}
	err = p.privateService.TxPool().AddTx(tx)
	if err != nil {
		log.Error("Fail to add Kardia's tx", "error", err)
		return configs.ErrAddKardiaTx
	}
	log.Info("Submit external private chain tx successfully", "txhash", tx.Hash().Hex())
	return nil
}

// ComputeTxMetadata pre-computes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (p *PermissionedProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	var (
		tx  *types.Transaction
		err error
	)
	switch event.Data.TxMethod {
	case configs.KardiaForwardRequestFunction:
		tx, err = p.createTxFromKardiaForwardedRequest(event, p.privateService.TxPool())
	case configs.KardiaForwardResponseFunction:
		tx, err = p.createTxFromKardiaForwardedResponse(event, p.privateService.TxPool())
	default:
		return nil, configs.ErrUnsupportedMethod
	}
	if err != nil {
		return nil, err
	}
	return &types.TxMetadata{
		TxHash: tx.Hash(),
		Target: types.BlockchainSymbol(strconv.FormatUint(*p.privateChainID, 10)),
	}, nil
	return nil, nil
}

func (p *PermissionedProxy) createTxFromKardiaForwardedRequest(event *types.EventData, pool *tx_pool.TxPool) (*types.Transaction, error) {
	if event.Data.TxMethod != configs.KardiaForwardRequestFunction {
		return nil, configs.ErrUnsupportedMethod
	}
	if event.Data.ExtData == nil || len(event.Data.ExtData) < configs.KardiaForwardRequestFields {
		log.Error("Event doesn't contains enough data")
		return nil, errInsufficientForwardRequestData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardRequestEmailIndex]) {
		log.Error("Missing email")
		return nil, errInsufficientForwardRequestData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardRequestFromOrgIndex]) {
		log.Error("Missing fromOrgId")
		return nil, errInsufficientForwardRequestData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardRequestToOrgIndex]) {
		log.Error("Missing toOrgId")
		return nil, errInsufficientForwardRequestData
	}
	// We don't process further if we are not targeted chain of the request
	toOrgID := string(event.Data.ExtData[configs.KardiaForwardRequestToOrgIndex])
	log.Error("orgID compare: ", "toOrgID", toOrgID, "private chain ID", strconv.FormatUint(*p.privateChainID, 10))
	if toOrgID != strconv.FormatUint(*p.privateChainID, 10) {
		return nil, errInvalidTargetChainId
	}
	tx, err := p.candidateSmcUtil.AddRequest(string(event.Data.ExtData[configs.KardiaForwardRequestEmailIndex]),
		string(event.Data.ExtData[configs.KardiaForwardRequestFromOrgIndex]), pool.State())
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// createTxFromKardiaForwardedResponse returns tx to call to private chain candidate smart contract to respond to a
// candidate info request
func (p *PermissionedProxy) createTxFromKardiaForwardedResponse(event *types.EventData, pool *tx_pool.TxPool) (*types.Transaction, error) {
	if event.Data.TxMethod != configs.KardiaForwardResponseFunction {
		return nil, configs.ErrUnsupportedMethod
	}
	if event.Data.ExtData == nil || len(event.Data.ExtData) < configs.KardiaForwardResponseFields {
		log.Error("Event doesn't contains enough data")
		return nil, errInsufficientForwardResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardResponseEmailIndex]) {
		log.Error("Missing email")
		return nil, errInsufficientForwardResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardResponseResponseIndex]) {
		log.Error("Missing response content")
		return nil, errInsufficientForwardResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardResponseFromOrgIndex]) {
		log.Error("Missing fromOrgId")
		return nil, errInsufficientForwardResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.KardiaForwardResponseToOrgIndex]) {
		log.Error("Missing toOrgId")
		return nil, errInsufficientForwardResponseData
	}
	toOrgID := string(event.Data.ExtData[configs.KardiaForwardResponseToOrgIndex])
	// We don't add response to our smart contract as we are not targeted chain of a response
	if toOrgID != strconv.FormatUint(*p.privateChainID, 10) {
		log.Error("We are not targeted chain of this response", "toOrgID", toOrgID)
		return nil, errInvalidTargetChainId
	}
	tx, err := p.candidateSmcUtil.AddExternalResponse(string(event.Data.ExtData[configs.KardiaForwardResponseEmailIndex]),
		string(event.Data.ExtData[configs.KardiaForwardResponseResponseIndex]),
		string(event.Data.ExtData[configs.KardiaForwardResponseFromOrgIndex]), pool.State())
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (p *PermissionedProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	p.internalChain = internalChain
}
