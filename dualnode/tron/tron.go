package tron

import (
	"strings"

	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/dev"
	"math/big"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"strconv"
	"errors"
	message2 "github.com/kardiachain/go-kardia/dualnode/message"
)


type Proxy struct {

	logger log.Logger // Logger for Tron service

	kardiaBc   base.BaseBlockChain
	txPool     *tx_pool.TxPool
	smcAddress *common.Address
	smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool // Event pool of DUAL service.

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// Chain head subscription for new blocks.
	chainHeadCh  chan base.ChainHeadEvent
	chainHeadSub event.Subscription

	// Queue configuration
	publishedEndpoint string
	queueTopic string
}

func NewProxy(
		kardiaBc base.BaseBlockChain,
		txPool *tx_pool.TxPool,
		dualBc base.BaseBlockChain,
		dualEventPool *event_pool.EventPool,
		smcAddr *common.Address,
		smcABIStr string,
		subscribedEndpoint *string,
		publishedEndpoint *string,
	) (*Proxy, error) {

	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(ServiceName)

	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}
	processor := &Proxy{
		logger: logger,
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,
		smcAddress: smcAddr,
		smcABI:     &smcABI,

		chainHeadCh: make(chan base.ChainHeadEvent, 5),
		queueTopic: ServiceName,
	}

	if publishedEndpoint != nil {
		processor.publishedEndpoint = *publishedEndpoint
	} else {
		processor.publishedEndpoint = configs.DefaultPublishedEndpoint
	}

	return processor, nil
}

func (n *Proxy) AddEvent(dualEvent *types.DualEvent) error {
	return n.eventPool.AddEvent(dualEvent)
}

func (n *Proxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	n.internalChain = internalChain
}

func (n *Proxy) SubmitTx(event *types.EventData) error {
	statedb, err := n.kardiaBc.State()
	if err != nil {
		log.Error("Cannot get kardia statedb", "err", err)
		return configs.ErrFailedGetState
	}

	switch event.Data.TxMethod {
	case configs.AddOrderFunction:
		if len(event.Data.ExtData) != configs.ExchangeV2NumOfExchangeDataField {
			return configs.ErrInsufficientExchangeData
		}
		senderAddr := common.HexToAddress(dev.MockSmartContractCallSenderAccount)
		originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])
		// Get all matched orders of newly added order
		releases, err := utils.CallKardiGetMatchingResultByTxId(senderAddr, n.kardiaBc, statedb, originalTx)
		if err != nil {
			return err
		}
		log.Info("Release info", "release", releases)
		if releases != "" {
			fields := strings.Split(releases, configs.ExchangeV2ReleaseFieldsSeparator)
			if len(fields) != 4 {
				log.Error("Invalid numn of field", "release", releases)
				return errors.New("invalid num of field for release")
			}
			// Parse release info into arrays of types, addresses, amounts and txids
			arrTypes := strings.Split(fields[configs.ExchangeV2ReleaseToTypeIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAddresses := strings.Split(fields[configs.ExchangeV2ReleaseAddressesIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAmounts := strings.Split(fields[configs.ExchangeV2ReleaseAmountsIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrTxIds := strings.Split(fields[configs.ExchangeV2ReleaseTxIdsIndex], configs.ExchangeV2ReleaseValuesSepatator)

			for i, t := range arrTypes {
				if t == configs.TRON { // If the receiving end if the release is TRX, send release request to TRON solidity Node through zeroMQ
					if arrAmounts[i] == "" || arrAddresses[i] == "" || arrTxIds[i] == "" {
						log.Error("Missing release info", "matchedTxId", arrTxIds[i], "field", i, "releases", releases)
						continue
					}
					address := arrAddresses[i]
					amount, err1 := strconv.ParseInt(arrAmounts[i], 10, 64) //big.NewInt(0).SetString(arrAmounts[i], 10)
					log.Info("Amount", "amount", amount, "in string", arrAmounts[i])
					if err1 != nil {
						log.Error("Error parse amount", "amount", arrAmounts[i])
						continue
					}
					// Divide amount from smart contract by 10^6 to get base TRX amount to release
					amountTron := big.NewInt(amount).Div(big.NewInt(amount), utils.TenPoweredBySix)

					// publish released data to zeroMQ
					// create a triggeredMessage and send it through ZeroMQ with topic KARDIA_CALL
					message := message2.TriggerMessage{
						ContractAddress: address, // TODO: this must be target contract address. Eg: Tron contract addr.
						MethodName: "release",
						Params: []string{address, amountTron.String()},
						CallBacks: []*message2.TriggerMessage{
							{
								ContractAddress: configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex).Hex(),
								MethodName: "updateTargetTx",
								Params: []string{
									// original tx, callback will be called after dual finish execute method,
									// txid after method is executed will be appended
									// callback method will be sent through 0MQ with DUAL_CALL topic
									arrTxIds[i],
								},
							},
						},
					}
					utils.PublishMessage(n.publishedEndpoint, KARDIA_CALL, message.String())
				}
			}

			return nil
		}
		return nil
	default:
		log.Warn("Unexpected method in TRON SubmitTx", "method", event.Data.TxMethod)
		return configs.ErrUnsupportedMethod
	}
}


