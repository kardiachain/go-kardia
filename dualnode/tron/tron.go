package tron

import (
	"strings"
	"strconv"
	"errors"
	"math/big"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	message2 "github.com/kardiachain/go-kardia/dualnode/message"
)

// TODO(@kiendn): remove it when we can return it from kardia master smart contract.
const EXCHANGE_ADDRESS = "413b4c5dfdd72d4795b31c62fc006525d1bb9d85fb"

type Proxy struct {

	logger log.Logger // Logger for Tron service

	kardiaBc   base.BaseBlockChain
	txPool     *tx_pool.TxPool

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
		publishedEndpoint string,
	) (*Proxy, error) {

	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(ServiceName)

	processor := &Proxy{
		logger: logger,
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,

		chainHeadCh: make(chan base.ChainHeadEvent, 5),
		queueTopic: ServiceName,
	}

	if publishedEndpoint != "" {
		processor.publishedEndpoint = publishedEndpoint
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
				log.Error("Invalid num of field", "release", releases)
				return errors.New("invalid num of field for release")
			}
			// Parse release info into arrays of types, addresses, amounts and txids
			arrTypes := strings.Split(fields[configs.ExchangeV2ReleaseToTypeIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAddresses := strings.Split(fields[configs.ExchangeV2ReleaseAddressesIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrAmounts := strings.Split(fields[configs.ExchangeV2ReleaseAmountsIndex], configs.ExchangeV2ReleaseValuesSepatator)
			arrTxIds := strings.Split(fields[configs.ExchangeV2ReleaseTxIdsIndex], configs.ExchangeV2ReleaseValuesSepatator)

			for i, t := range arrTypes {
				if t == configs.TRON {
					// if the release is TRX, send release request to TRON solidity Node through 0mq
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
					// Divide amount from smart contract by 10^6 to get base TRX amount
					amountTron := big.NewInt(amount).Div(big.NewInt(amount), utils.TenPoweredBySix)

					// release tron to receiver
					n.Release(address, amountTron.String(), arrTxIds[i])
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

// Release releases TRX to receiver, txId is kardiaTxId which is used for callback method.
func (n *Proxy) Release(receiver, txId, amount string) {
	// publish released data to zeroMQ
	// create a triggeredMessage and send it through ZeroMQ with topic KARDIA_CALL
	message := message2.TriggerMessage{
		// TODO (@kiendn): this must be target contract address. Eg: Tron contract addr.
		// TODO (@kiendn): return it from ExtData
		ContractAddress: EXCHANGE_ADDRESS,
		MethodName: "release",
		Params: []string{receiver, amount},
		CallBacks: []*message2.TriggerMessage{
			{
				ContractAddress: configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex).Hex(),
				MethodName: "updateTargetTx",
				Params: []string{
					// original tx, callback will be called after dual finish execute method,
					// txid after dual finish executing will be appended
					// callback method will be sent through 0MQ with DUAL_CALL topic
					txId,
				},
			},
		},
	}
	utils.PublishMessage(n.publishedEndpoint, KARDIA_CALL, message)
}

// when we submitTx to externalChain, so I simply return a basic metadata here basing on target and event hash,
// to differentiate TxMetadata inferred from events
func (n *Proxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.KARDIA,
	}, nil
}


