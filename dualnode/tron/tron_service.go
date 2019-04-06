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

package tron


import (
	"fmt"
	"math/big"
	"github.com/pebbe/zmq4"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/rpc"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/dualnode"
	"github.com/kardiachain/go-kardia/dualnode/message"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
)

const (
	ServiceName = "TRON"
	NetworkID = 300
	KARDIA_CALL = "KARDIA_CALL"
	DUAL_CALL = "DUAL_CALL"
	DUAL_MSG = "DUAL_MSG"
)


// Service implements Service for running tron dual node, including essential APIs
type Service struct {
	logger log.Logger // Logger for Tron service

	// Channel for shutting down the service
	shutdownChan chan bool

	dualBlockchain *blockchain.DualBlockChain
	dualEventPool  *event_pool.EventPool
	internalChain  base.BlockChainAdapter

	// txPool of kardiachain
	txPool     *tx_pool.TxPool

	networkID uint64

	Apis []rpc.API
}

// newNeoService creates a new NeoService object (including the
// initialisation of the NeoService object)
func newService() (*Service, error) {
	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(ServiceName)

	tronService := &Service{
		logger:       logger,
		shutdownChan: make(chan bool),
		networkID:    NetworkID,
	}

	return tronService, nil
}

// Returns a new NeoService
func NewService(ctx *node.ServiceContext) (node.Service, error) {
	service, err := newService()
	if err != nil {
		return nil, err
	}
	return service, nil
}

// Initialize sets up blockchains and event pool for NeoService
func (s *Service) Initialize(internalBlockchain base.BlockChainAdapter, dualchain *blockchain.DualBlockChain,
	pool *event_pool.EventPool, txPool *tx_pool.TxPool, subscribedEndpoint string) {
	s.internalChain = internalBlockchain
	s.dualEventPool = pool
	s.dualBlockchain = dualchain
	s.txPool = txPool

	// start new subscribed channel from 0mq
	go s.StartSubscribe(subscribedEndpoint)
}

func (s *Service) StartSubscribe(subscribedEndpoint string) {

	endpoint := configs.DefaultSubscribedEndpoint

	if subscribedEndpoint != "" {
		endpoint = subscribedEndpoint
	}

	subscriber, _ := zmq4.NewSocket(zmq4.SUB)
	defer subscriber.Close()
	subscriber.Connect(endpoint)

	for {
		//  Read envelope with address
		topic, _ := subscriber.Recv(0)
		//  Read message contents
		contents, _ := subscriber.Recv(0)
		fmt.Printf("[%s] %s\n", topic, contents)

		if err := s.MessageHandler(topic, contents); err != nil {
			s.logger.Error("Error while creating new event", "err", err.Error())
		}
	}
}

// MessageHandler handles messages come from dual to kardia
func (s *Service) MessageHandler(topic, message string) error {
	switch topic {
	case DUAL_CALL:
		// callback from dual
		triggerMessage := message.TriggerMessage{}
		triggerMessage.XXX_Unmarshal([]byte(message))

		tx, err := utils.ExecuteKardiaSmartContract(s.txPool.State(), triggerMessage.ContractAddress, triggerMessage.MethodName, triggerMessage.Params)
		if err != nil {
			return err
		}

		if err := s.txPool.AddLocal(tx); err != nil {
			return nil
		}

	case DUAL_MSG:
		// message from dual after it catches a triggered smc tx
		// unpack contents to DualMessage
		dualMessage := message.Message{}
		dualMessage.XXX_Unmarshal([]byte(message))

		// TODO: this is used for exchange demo, remove the condition whenever we have dynamic handler method for this
		if dualMessage.MethodName == configs.ExternalDepositFunction {
			return s.NewEvent(dualMessage)
		}
	}
	return nil
}

// NewEvent receives data from Tron where encodedMsg is used for validating the message
// returns error in case event cannot be added to eventPool
func (s *Service) NewEvent(dualMsg message.Message) error {
	dualState, err := s.dualBlockchain.State()
	if err != nil {
		log.Error("Fail to get TRXKardia state", "error", err)
		return err
	}

	// TODO: This is used for exchange use case, will remove this after applying dynamic method
	receiver := []byte(dualMsg.GetParams()[0])
	pair := dualMsg.GetParams()[1]

	from, to, err := utils.GetExchangePair(pair)
	if err != nil {
		return nil
	}

	txHash := common.HexToHash(dualMsg.GetTransactionId())
	nonce := dualState.GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	// Compose extraData struct for fields related to exchange from data extracted by Neo event
	extraData := make([][]byte, configs.ExchangeV2NumOfExchangeDataField)
	extraData[configs.ExchangeV2SourcePairIndex] = []byte(*from)
	extraData[configs.ExchangeV2DestPairIndex] = []byte(*to)
	extraData[configs.ExchangeV2SourceAddressIndex] = []byte(dualMsg.GetSender())
	extraData[configs.ExchangeV2DestAddressIndex] = receiver
	extraData[configs.ExchangeV2OriginalTxIdIndex] = []byte(dualMsg.GetTransactionId())
	extraData[configs.ExchangeV2AmountIndex] = big.NewInt(int64(dualMsg.GetAmount())).Bytes()
	extraData[configs.ExchangeV2TimestampIndex] = big.NewInt(int64(dualMsg.GetTimestamp())).Bytes()

	eventSummary := &types.EventSummary{
		TxMethod: dualMsg.MethodName,
		TxValue:  big.NewInt(int64(dualMsg.Amount)),
		ExtData:  extraData,
	}

	actionsTmp := [...]*types.DualAction{
		&types.DualAction{
			Name: dualnode.CreateKardiaMatchAmountTx,
		},
	}

	dualEvent := types.NewDualEvent(nonce, true /* internalChain */, types.TRON, &txHash, eventSummary, &types.DualActions{
		Actions: actionsTmp[:],
	})

	// Compose extraData struct for fields related to exchange
	txMetaData, err := s.internalChain.ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error compute internal tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetaData
	err = s.dualEventPool.AddEvent(dualEvent)
	if err != nil {
		log.Error("Failed to add dual event to pool", "err", err)
		return err
	}
	log.Info("Added to dual event pool successfully", "eventHash", dualEvent.Hash().String())
	return nil
}

func (s *Service) NetVersion() uint64 { return s.networkID }

func (s *Service) APIs() []rpc.API {
	return []rpc.API{}
}

// Protocols implements Service, returning all the currently configured
// network protocols to start.
func (s *Service) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (s *Service) SetApis(apis []rpc.API) {
	if len(apis) > 0 {
		for _, api := range apis {
			s.Apis = append(s.Apis, api)
		}
	}
}

func (s *Service) Start(server *p2p.Server) error {
	return nil
}

func (s *Service) Stop() error {
	return nil
}

