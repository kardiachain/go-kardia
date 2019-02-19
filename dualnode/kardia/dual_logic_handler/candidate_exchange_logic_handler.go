package dual_logic_handler

import (
	"errors"
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

var ErrInsufficientCandidateRequestData = errors.New("insufficient candidate request data")
var ErrInsufficientCandidateResponseData = errors.New("insufficient candidate response data")
var ErrUnpackForwardRequestInfo = errors.New("error unpacking info forward request input")
var ErrUnpackForwardResponseInfo = errors.New("error unpacking info forward response input")

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

type CandidateExchangeLogicHandler struct {
	smcAddress *common.Address
	smcABI     *abi.ABI
	externalChain base.BlockChainAdapter
}

func NewCandidateExchangeLogicHandler(smcAddr *common.Address, smcABI *abi.ABI) (*CandidateExchangeLogicHandler, error) {
	handler := &CandidateExchangeLogicHandler{smcAddress: smcAddr, smcABI: smcABI}
	return handler, nil
}

// RegisterExternalChain attaches an external blockchain adapter for this handler
func (h *CandidateExchangeLogicHandler) RegisterExternalChain(externalChain base.BlockChainAdapter) {
	h.externalChain = externalChain
}

// SubmitTx submits a tx to kardia to notify a candidate info requested from external chain
func (h *CandidateExchangeLogicHandler) SubmitTx(event *types.EventData, blockchain base.BaseBlockChain, txPool *tx_pool.TxPool) error {
	log.Error("Submit to Kardia", "value", event.Data.TxValue, "method", event.Data.TxMethod)
	var (
		tx  *types.Transaction
		err error
	)
	switch event.Data.TxMethod {
	case configs.PrivateChainRequestInfoFunction:
		// There is a request comes from external chain, we create a tx to forward it to Kardia candidate exchange smc
		tx, err = h.createTxFromExternalRequestData(event, txPool)
	case configs.PrivateChainCompleteRequestFunction:
		// There is a response comes from external chain, we create a tx to forward it to Kardia candidate exchange smc
		tx, err = h.createTxFromExternalResponseData(event, txPool)
	default:
		log.Error("Unsupported method", "method", event.Data.TxMethod)
		return configs.ErrUnsupportedMethod
	}
	if err != nil {
		log.Error("Fail to create Kardia's tx from DualEvent", "err", err)
		return configs.ErrCreateKardiaTx
	}
	err = txPool.AddLocal(tx)
	if err != nil {
		log.Error("Fail to add Kardia's tx", "error", err)
		return configs.ErrAddKardiaTx
	}
	log.Info("Submit Kardia's tx successfully", "txhash", tx.Hash().Hex())
	return nil
}

// createTxFromExternalRequestData parses event data to create tx to Kardia candidate exchange smart contract to
// forward a request
func (h *CandidateExchangeLogicHandler) createTxFromExternalRequestData(event *types.EventData, pool *tx_pool.TxPool) (*types.Transaction, error) {
	if event.Data.TxMethod != configs.PrivateChainRequestInfoFunction {
		return nil, configs.ErrUnsupportedMethod
	}
	if event.Data.ExtData == nil || len(event.Data.ExtData) < configs.PrivateChainCandidateRequestFields {
		log.Error("Event doesn't contains enough data")
		return nil, ErrInsufficientCandidateRequestData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestEmailIndex]) {
		log.Error("Missing email from external request data")
		return nil, ErrInsufficientCandidateRequestData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestFromOrgIndex]) {
		log.Error("Missing fromOrgId from external request data")
		return nil, ErrInsufficientCandidateRequestData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestToOrgIndex]) {
		log.Error("Missing toOrgId from external request data")
		return nil, ErrInsufficientCandidateRequestData
	}
	tx, err := utils.CreateForwardRequestTx(string(event.Data.ExtData[configs.PrivateChainCandidateRequestEmailIndex]),
		string(event.Data.ExtData[configs.PrivateChainCandidateRequestFromOrgIndex]),
		string(event.Data.ExtData[configs.PrivateChainCandidateRequestToOrgIndex]), pool.State())
	return tx, err
}

// createTxFromExternalRequestData parses event data to create tx to Kardia candidate exchange smart contract to
// forward a response
func (h *CandidateExchangeLogicHandler) createTxFromExternalResponseData(event *types.EventData, pool *tx_pool.TxPool) (*types.Transaction, error) {
	if event.Data.TxMethod != configs.PrivateChainCompleteRequestFunction {
		return nil, configs.ErrUnsupportedMethod
	}
	if event.Data.ExtData == nil || len(event.Data.ExtData) < configs.PrivateChainCandidateRequestCompletedFields {
		log.Error("Event doesn't contains enough data")
		return nil, ErrInsufficientCandidateResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedEmailIndex]) {
		log.Error("Missing email from external response data")
		return nil, ErrInsufficientCandidateResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedContentIndex]) {
		log.Error("Missing content from external response data")
		return nil, ErrInsufficientCandidateResponseData
	}
	if utils.IsNilOrEmpty(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedToOrgIDIndex]) {
		log.Error("Missing to org ID from external response data")
		return nil, ErrInsufficientCandidateResponseData
	}
	tx, err := utils.CreateForwardResponseTx(string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedEmailIndex]),
		string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedContentIndex]),
		string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedFromOrgIDIndex]),
		string(event.Data.ExtData[configs.PrivateChainCandidateRequestCompletedToOrgIDIndex]), pool.State())
	return tx, err
}

// ComputeTxMetadata computes metadata of tx that will be submitted to Kardia master smart contract to notify other external
// chain about requested candidate info
func (h *CandidateExchangeLogicHandler) ComputeTxMetadata(event *types.EventData, txPool *tx_pool.TxPool) (*types.TxMetadata, error) {
	var (
		tx  *types.Transaction
		err error
	)
	switch event.Data.TxMethod {
	case configs.PrivateChainRequestInfoFunction:
		tx, err = h.createTxFromExternalRequestData(event, txPool)
	case configs.PrivateChainCompleteRequestFunction:
		tx, err = h.createTxFromExternalResponseData(event, txPool)
	default:
		return nil, configs.ErrUnsupportedMethod
	}
	if err != nil {
		return nil, err
	}
	return &types.TxMetadata{
		TxHash: tx.Hash(),
		Target: types.KARDIA,
	}, nil
}

// ExtractKardiaTxSummary extracts information from Kardia tx input about candidate info request / response forwarded
func (h *CandidateExchangeLogicHandler) ExtractKardiaTxSummary(tx *types.Transaction) (types.EventSummary, error) {
	method, err := h.smcABI.MethodById(tx.Data()[0:4])
	if err != nil {
		log.Error("Fail to unpack smc update method in tx", "tx", tx, "error", err)
		return types.EventSummary{}, err
	}
	input := tx.Data()
	switch method.Name {
	case configs.KardiaForwardRequestFunction:
		candidateRequestData := make([][]byte, configs.KardiaForwardRequestFields)
		var incomingRequest KardiaForwardRequestInput
		err = h.smcABI.UnpackInput(&incomingRequest, configs.KardiaForwardRequestFunction, input[4:])
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
		err = h.smcABI.UnpackInput(&forwardResponseInput, configs.KardiaForwardResponseFunction, input[4:])
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
func (h *CandidateExchangeLogicHandler) HandleKardiaTx(tx *types.Transaction, eventPool *event_pool.EventPool,
	txPool *tx_pool.TxPool) error {
	eventSummary, err := h.ExtractKardiaTxSummary(tx)
	if err != nil {
		log.Error("Error when extracting Kardia main chain's tx summary.")
	}
	if eventSummary.TxMethod != configs.KardiaForwardResponseFunction && eventSummary.TxMethod != configs.KardiaForwardRequestFunction {
		log.Info("Skip tx updating smc not related to candidate exchange", "method", eventSummary.TxMethod)
	}
	log.Info("Detect Kardia's tx updating smc", "method", eventSummary.TxMethod, "value",
		eventSummary.TxValue, "hash", tx.Hash())
	nonce := eventPool.State().GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	kardiaTxHash := tx.Hash()
	txHash := common.BytesToHash(kardiaTxHash[:])
	dualEvent := types.NewDualEvent(nonce, false, types.KARDIA, &txHash, &eventSummary)
	txMetadata, err := h.externalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("method:", "method", eventSummary.TxMethod)
		log.Error("Error computing tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetadata
	log.Info("Create DualEvent for Kardia's Tx", "dualEvent", dualEvent)
	err = eventPool.AddEvent(dualEvent)
	if err != nil {
		log.Error("Fail to add dual's event", "error", err)
		return err
	}
	log.Info("Submitted Kardia's DualEvent to event pool successfully", "txHash", tx.Hash().String(),
		"eventHash", dualEvent.Hash().String())
	return nil
}

func (h *CandidateExchangeLogicHandler) GetSmcAddress() common.Address {
	return *h.smcAddress
}

func (h *CandidateExchangeLogicHandler) Init(pool *tx_pool.TxPool) error {
	return nil
}
