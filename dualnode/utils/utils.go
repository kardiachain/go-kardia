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

package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kardiachain/go-kardia/ksml"
	message2 "github.com/kardiachain/go-kardia/ksml/proto"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"

	"github.com/golang/protobuf/jsonpb"
	dualMsg "github.com/kardiachain/go-kardia/dualnode/message"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	"github.com/pebbe/zmq4"
)

const (
	KARDIA_CALL = "KARDIA_CALL"
	DUAL_CALL   = "DUAL_CALL"
	DUAL_MSG    = "DUAL_MSG"
)

// PublishMessage publishes message to 0MQ based on given endpoint, topic
func PublishMessage(endpoint, topic string, message dualMsg.TriggerMessage) error {
	pub, _ := zmq4.NewSocket(zmq4.PUB)
	defer pub.Close()
	pub.Connect(endpoint)

	// sleep 1 second to prevent socket closes
	time.Sleep(1 * time.Second)

	// send topic
	if _, err := pub.Send(topic, zmq4.SNDMORE); err != nil {
		return err
	}
	m := &jsonpb.Marshaler{}
	msgToSend, err := m.MarshalToString(&message)
	if err != nil {
		log.Error("Failed to encode", "message", message.String(), "error", err)
	}
	// send message
	log.Info("Publish message", "topic", topic, "msgToSend", msgToSend)
	if _, err := pub.Send(msgToSend, zmq4.DONTWAIT); err != nil {
		return err
	}
	return nil
}

// ExecuteKardiaSmartContract executes smart contract based on address, method and list of params
func ExecuteKardiaSmartContract(txPool *tx_pool.TxPool, bc base.BaseBlockChain, contractAddress, methodName string, params []string) (*types.Transaction, error) {
	// find contractAddress, to see if it is saved in chain or not.
	db := bc.DB()

	// make sure contractAddress has prefix 0x
	if contractAddress[:2] != "0x" {
		contractAddress = "0x" + contractAddress
	}
	kAbi := rawdb.ReadSmartContractAbi(db, contractAddress)
	if kAbi == nil {
		return nil, fmt.Errorf("cannot find abi from smc address: %v", contractAddress)
	}
	convertedParam := make([]interface{}, 0)
	for _, v := range params {
		convertedParam = append(convertedParam, v)
	}
	input, err := kAbi.Pack(methodName, convertedParam...)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to pack methodName=%v params=%v err=%v", methodName, params, err))
		return nil, err
	}
	if bc.P2P() != nil {
		sender := bc.P2P().Address()
		currentHeader := bc.CurrentHeader()
		stateDb := txPool.State()
		gasUsed, err := ksml.EstimateGas(*sender, common.HexToAddress(contractAddress), currentHeader, bc, stateDb, input)
		if err != nil {
			return nil, err
		}
		nonce := txPool.Nonce(*sender)
		return ksml.GenerateSmcCall(nonce, bc.P2P().PrivKey(), common.HexToAddress(contractAddress), input, gasUsed)
	}
	return nil, fmt.Errorf("cannot execute kardia smart contract - base account not found")
}

// MessageHandler handles messages come from dual to kardia
func MessageHandler(proxy base.BlockChainAdapter, topic, message string) error {
	proxy.Logger().Info("Starting MessageHandler", "topic", topic)
	switch topic {
	case DUAL_CALL:
		// callback from dual
		triggerMessage := dualMsg.TriggerMessage{}
		if err := jsonpb.UnmarshalString(message, &triggerMessage); err != nil {
			proxy.Logger().Error("Error on unmarshal triggerMessage", "err", err, "topic", topic)
			return err
		}
		if len(triggerMessage.Params) == 0 {
			return fmt.Errorf("invalid trigger message: contractAddress %v, Method %v", triggerMessage.ContractAddress, triggerMessage.MethodName)
		}
		proxy.Logger().Info(
			"DUAL_CALL",
			"contractAddress", triggerMessage.ContractAddress,
			"methodName", triggerMessage.MethodName,
			"params", triggerMessage.Params,
		)
		tx, err := ExecuteKardiaSmartContract(proxy.KardiaTxPool(), proxy.KardiaBlockChain(), triggerMessage.ContractAddress, triggerMessage.MethodName, triggerMessage.Params)
		if err != nil {
			proxy.Logger().Error("Error on executing kardia smart contract", "err", err, "topic", topic)
			return err
		}

		if err := proxy.KardiaTxPool().AddLocal(tx); err != nil {
			proxy.Logger().Error("Error on adding tx to txPool", "err", err, "topic", topic)
			return err
		}

	case DUAL_MSG:
		// message from dual after it catches a triggered smc tx
		// unpack contents to DualMessage
		msg := dualMsg.Message{}
		if err := jsonpb.UnmarshalString(message, &msg); err != nil {
			proxy.Logger().Error("Error decoding", "message", message)
			if e, ok := err.(*json.SyntaxError); ok {
				proxy.Logger().Error("Error syntax at", "byte offset", e.Offset)
			}
			return err
		}

		// get contract address in dual proxy to check if contract address exists or not. If not do nothing.
		// if it does, try to get watcher event in db by its contract address and methodName

		// make sure contractAddress has prefix 0x
		contractAddress := msg.ContractAddress
		if contractAddress[:2] != "0x" {
			contractAddress = "0x" + contractAddress
		}
		watcher := rawdb.ReadEvent(proxy.DualBlockChain().DB(), contractAddress, msg.MethodName)
		if watcher != nil {
			// get kardia master smc from dualAction
			smc, _ := rawdb.ReadEvents(proxy.DualBlockChain().DB(), contractAddress)
			if smc == "" {
				return fmt.Errorf("cannot find dualAction from watcherAction %v", watcher.Method)
			}
			masterSmc := common.HexToAddress(smc)
			to := msg.GetParams()[1]
			from := proxy.Name()
			txHash := common.HexToHash(msg.GetTransactionId())
			// new event message
			eventMessage := &message2.EventMessage{
				MasterSmartContract: masterSmc.Hex(),
				TransactionId:       msg.GetTransactionId(),
				From:                from,
				To:                  to,
				Method:              msg.MethodName,
				Params:              msg.Params,
				Amount:              msg.GetAmount(),
				Sender:              msg.GetSender(),
				BlockNumber:         msg.BlockNumber,
				Timestamp:           time.Unix(int64(msg.Timestamp), 10),
			}
			if watcher.WatcherActions != nil && len(watcher.WatcherActions) > 0 {
				parser := ksml.NewParser(
					proxy.Name(),
					proxy.PublishedEndpoint(),
					PublishMessage,
					proxy.KardiaBlockChain(),
					proxy.KardiaTxPool(),
					&masterSmc,
					watcher.WatcherActions,
					eventMessage,
					false,
				)
				if err := parser.ParseParams(); err != nil {
					return err
				}
				params, err := parser.GetParams()
				if err != nil {
					return err
				}
				eventMessage.Params = append(eventMessage.Params, params...)
			}
			return NewEvent(proxy, msg.BlockNumber, eventMessage, txHash, watcher.DualActions, true)
		}
		proxy.Logger().Debug("watcher not found", "contractAddress", contractAddress, "method", msg.MethodName)
	}
	return nil
}

// StartSubscribe subscribes messages from subscribedEndpoint
func StartSubscribe(proxy base.BlockChainAdapter) {
	subscriber, _ := zmq4.NewSocket(zmq4.SUB)
	defer subscriber.Close()
	subscriber.Bind(proxy.SubscribedEndpoint())
	subscriber.SetSubscribe("")
	time.Sleep(time.Second)
	for {
		if err := subscribe(subscriber, proxy); err != nil {
			proxy.Logger().Error("Error while subscribing", "err", err.Error())
		}
	}
}

// subscribe handles getting/handle topic and content, return error if any
func subscribe(subscriber *zmq4.Socket, proxy base.BlockChainAdapter) error {
	proxy.Lock()
	defer proxy.UnLock()

	//  Read envelope with address
	topic, err := subscriber.Recv(0)
	if err != nil {
		return err
	}
	//  Read message contents
	contents, err := subscriber.Recv(0)
	if err != nil {
		return err
	}
	proxy.Logger().Info("[%s] %s\n", topic, contents)

	if err := MessageHandler(proxy, topic, contents); err != nil {
		proxy.Logger().Error("Error while creating new event", "err", err.Error())
		return err
	}
	return nil
}

// NewEvent creates new event and add to eventPool
func NewEvent(proxy base.BlockChainAdapter, blockHeight uint64, msg *message2.EventMessage, txHash common.Hash, actions []string, fromExternal bool) error {
	if proxy.DualBlockChain().P2P().Address() == nil {
		return fmt.Errorf("current node does not have base account to create new event")
	}

	privateKey := proxy.DualBlockChain().P2P().PrivKey()
	dualEvent := types.NewDualEvent(blockHeight, fromExternal /* internalChain */, types.BlockchainSymbol(proxy.Name()), &txHash, msg, actions)

	// Compose extraData struct for fields related to exchange
	txMetaData, err := proxy.InternalChain().ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error compute internal tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetaData
	signedEvent, err := types.SignEvent(dualEvent, privateKey)
	if err != nil {
		return err
	}
	proxy.Logger().Info("Adding new event", "evt", signedEvent.String(), "hash", dualEvent.Hash().Hex())
	if err := proxy.DualEventPool().AddEvent(signedEvent); err != nil {
		proxy.Logger().Error("error while adding dual event", "err", err, "event", signedEvent.Hash().Hex())
		return err
	}
	log.Info("Added to dual event pool successfully", "eventHash", signedEvent.Hash().String())
	return nil
}
