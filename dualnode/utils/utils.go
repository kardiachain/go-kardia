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
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/mainchain/tx_pool"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pebbe/zmq4"
	"github.com/pkg/errors"

	"github.com/kardiachain/go-kardia/configs"
	dualMsg "github.com/kardiachain/go-kardia/dualnode/message"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
)

// TODO(@sontranrad): remove all of these constants for production

const (
	KARDIA_CALL = "KARDIA_CALL"
	DUAL_CALL   = "DUAL_CALL"
	DUAL_MSG    = "DUAL_MSG"
)

// TODO: note that when we have dynamic method, these values will be moved to smartcontract or anything that can handle this case.
var AvailableExchangeType = map[string]bool{
	configs.TRON: true,
	configs.NEO:  true,
	configs.ETH:  true,
}

var MaximumGasToCallStaticFunction = uint(4000000)
var errAbiNotFound = errors.New("ABI not found")
var errAmountLessThanOne = errors.New("Amount is less than one to send")
var errInvalidExchangeRate = errors.New("Invalid exchange rate")
var errInvalidSourceMatchAmount = errors.New("Invalid source for match amount tx")
var errErrorConvertRateFloat = errors.New("Error to convert rate to float")
var TenPoweredBySixFloat = big.NewFloat(float64(math.Pow10(6)))
var TenPoweredByEight = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(8), nil)
var TenPoweredByTen = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(10), nil)
var TenPoweredByTenFloat = big.NewFloat(float64(math.Pow10(10)))
var TenPoweredByTwelve = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(12), nil)
var TenPoweredByTwelveFloat = big.NewFloat(float64(math.Pow10(12)))

type MatchedRequest struct {
	MatchedRequestID *big.Int `abi:"matchedRequestID"`
	DestAddress      string   `abi:"destAddress"`
	SendAmount       *big.Int `abi:"sendAmount"`
}

// The following function is just call the master smc and return result in bytes format
func CallStaticKardiaMasterSmc(from common.Address, to common.Address, bc base.BaseBlockChain, input []byte, statedb *state.StateDB) (result []byte, err error) {
	context := vm.NewKVMContextFromDualNodeCall(from, bc.CurrentHeader(), bc)
	vmenv := kvm.NewKVM(context, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(MaximumGasToCallStaticFunction))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

// CallGetRate calls to Kardia exchange contract to get rate of a specific pair, return from amount and to amount
func CallGetRate(smc common.Address, kAbi abi.ABI, fromType string, toType string, bc base.BaseBlockChain, statedb *state.StateDB) (fromAmount *big.Int, receivedAmount *big.Int, err error) {

	senderAddr := bc.Config().BaseAccount.Address
	getRateInput, err := kAbi.Pack("getRate", fromType, toType)
	if err != nil {
		log.Error("Error packing get rate input", "err", err)
		// get default rate if error is thrown
		return configs.GetRateFromType(fromType), configs.GetRateFromType(toType), err
	}

	result, err := CallStaticKardiaMasterSmc(senderAddr, smc, bc, getRateInput, statedb)
	if err != nil {
		log.Error("Error call get rate", "err", err)
		// get default rate if error is thrown
		return configs.GetRateFromType(fromType), configs.GetRateFromType(toType), err
	}

	// init a rateStruct based on returned type from smart contract
	var rateStruct struct {
		FromAmount     *big.Int
		ReceivedAmount *big.Int
	}
	err = kAbi.Unpack(&rateStruct, "getRate", result)
	if err != nil {
		log.Error("Error unpack rate result", "err", err)
		// get default rate if error is thrown
		return configs.GetRateFromType(fromType), configs.GetRateFromType(toType), err
	}
	return rateStruct.FromAmount, rateStruct.ReceivedAmount, nil
}

// Creates a Kardia tx to report new matching amount from Eth/Neo/TRX network, return nil in case of any error occurs
func CreateKardiaMatchAmountTx(txPool *tx_pool.TxPool, smc common.Address, kAbi abi.ABI, quantity *big.Int, sourceAddress string,
	destinationAddress string, source string, destination string, hash string, bc base.BaseBlockChain) (*types.Transaction, error) {

	statedb := txPool.State()
	// check if source and destination types are valid or not.
	if !AvailableExchangeType[source] || !AvailableExchangeType[destination] {
		return nil, fmt.Errorf("invalid type")
	}

	var matchInput []byte
	timestamp := big.NewInt(time.Now().Unix())
	temp := big.NewInt(1)
	fromAmount, toAmount, err := CallGetRate(smc, kAbi, source, destination, bc, statedb)
	if err != nil {
		return nil, errInvalidExchangeRate
	}

	var convertedAmount *big.Int

	// unit of ordered amount will be based on the type which has smaller unit based.
	// for eg: int ETH-NEO, NEO has 10^8 while ETH has 10^18, hence the order amount will be based on NEO
	log.Info("Prepare for convert amount", "source", source, "destination", destination,
		"fromAmount", fromAmount, "toAmount", toAmount, "quantity", quantity)

	if fromAmount.Cmp(big.NewInt(0)) == 0 || toAmount.Cmp(big.NewInt(0)) == 0 {
		log.Error("Invalid exchange rate", "source", source, "destination", destination,
			"fromAmount", fromAmount, "toAmount", toAmount)
		return nil, errInvalidExchangeRate
	}
	switch source {
	case configs.ETH:
		convertedAmount = temp.Mul(quantity, fromAmount)
		convertedAmount = temp.Div(convertedAmount, toAmount)

		if destination == configs.NEO {
			convertedAmount = temp.Div(convertedAmount, TenPoweredByTen)
		} else if destination == configs.TRON {
			convertedAmount = temp.Div(convertedAmount, TenPoweredByTwelve)
		}
	case configs.NEO:
		if destination == configs.ETH {
			convertedAmount = temp.Mul(quantity, TenPoweredByEight)
		} else if destination == configs.TRON {
			// Convert rate to float
			rateFloat, err := ToRateFloat(fromAmount, toAmount, 6)
			if err != nil {
				log.Error("Error to convert rate to float", "error", err, "fromAmount", fromAmount, "toAmount", toAmount)
				return nil, errErrorConvertRateFloat
			}
			rateInt, _ := big.NewFloat(1).Mul(big.NewFloat(rateFloat), TenPoweredBySixFloat).Int64()
			convertedAmount = temp.Mul(big.NewInt(rateInt), quantity)
		}
	case configs.TRON:
		// currently TRON has smallest unit, therefore no need to calculate anything here.
		convertedAmount = quantity
	default:
		log.Error("Invalid source for match amount tx", "src", source)
		return nil, errInvalidSourceMatchAmount
	}

	log.Info("AddOrderFunction", "fromType", source, "toType", destination, "srcAddress", sourceAddress,
		"destAddress", destinationAddress, "originalTx", hash, "quantity", convertedAmount.String(), "timestamp", timestamp)

	matchInput, err = kAbi.Pack(configs.AddOrderFunction, source, destination, sourceAddress,
		destinationAddress, hash, convertedAmount, timestamp)

	if err != nil {
		log.Error("Error packing abi", "error", err, "address")
		return nil, err
	}
	return tool.GenerateSmcCall(&bc.Config().BaseAccount.PrivateKey, smc, matchInput, txPool, false), nil
}

func ToRateFloat(fromAmount *big.Int, toAmount *big.Int, precision int) (float64, error) {
	rateFloat := float64(fromAmount.Int64()) / float64(toAmount.Int64())
	format := "%." + strconv.Itoa(precision) + "f"
	rateRound, err := strconv.ParseFloat(fmt.Sprintf(format, rateFloat), 64)
	if err != nil {
		return 0, err
	}
	return rateRound, nil
}

func CallKardiGetMatchingResultByTxId(from common.Address, bc base.BaseBlockChain, statedb *state.StateDB, originalTx string) (string, error) {
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return "", err
	}
	var getMatchingResultByTxId []byte
	log.Info("CallKardiaGetMatchingResultByTxId", "originalTx", originalTx)
	getMatchingResultByTxId, err = kABI.Pack("getMatchingResult", originalTx)
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return "", err
	}
	result, err := CallStaticKardiaMasterSmc(from, masterSmcAddr, bc, getMatchingResultByTxId, statedb)
	if err != nil {
		log.Error("Error getMatchingResult", "err", err)
		return "", err
	}
	var matchingResult struct {
		Results string
	}
	err = kABI.Unpack(&matchingResult, "getMatchingResult", result)
	if err != nil {
		log.Error("Error unpack getMatchingResult", "err", err)
		return "", err
	}
	return matchingResult.Results, nil
}

func UpdateKardiaTx(txPool *tx_pool.TxPool, smartContract common.Address, kAbi abi.ABI, originalTx string, tx string, privateKey ecdsa.PrivateKey) (*types.Transaction, error) {
	completeInput, err := kAbi.Pack(configs.UpdateKardiaTx, originalTx, tx)
	if err != nil {
		log.Error("Failed to pack updateKardiaTx", "originalTx", originalTx, "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(&privateKey, smartContract, completeInput, txPool, true), nil
}

// CreateForwardRequestTx creates tx call to Kardia candidate exchange contract to forward a candidate request to another
// external chain
func CreateForwardRequestTx(email string, fromOrgId string, toOrgId string, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	exchangeSmcAddr, exchangeSmcAbi := configs.GetContractDetailsByIndex(configs.KardiaCandidateExchangeSmcIndex)
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
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), exchangeSmcAddr, requestInfoInput, txPool, false), nil
}

// CreateForwardResponseTx creates tx call to Kardia candidate exchange contract to fulfill a candidate info request
// from external private chain, receiving private chain will catch the event fired from Kardia exchange contract to process
// candidate info
func CreateForwardResponseTx(email string, response string, fromOrgId string, toOrgId string,
	txPool *tx_pool.TxPool) (*types.Transaction, error) {
	exchangeSmcAddr, exchangeSmcAbi := configs.GetContractDetailsByIndex(configs.KardiaCandidateExchangeSmcIndex)
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
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), exchangeSmcAddr, requestInfoInput, txPool, false), nil
}

// Return a common private key to call to Kardia smc from dual node
func GetPrivateKeyToCallKardiaSmc() *ecdsa.PrivateKey {
	addrKeyBytes, _ := hex.DecodeString(configs.KardiaPrivKeyToCallSmc)
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	return addrKey
}

func IsNilOrEmpty(data []byte) bool { return data == nil || string(data) == "" }

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

	kAbi := db.ReadSmartContractAbi(contractAddress)
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
	if bc.Config().BaseAccount != nil {
		return tool.GenerateSmcCall(&bc.Config().BaseAccount.PrivateKey, common.HexToAddress(contractAddress), input, txPool, false), nil
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

		if err := proxy.KardiaTxPool().AddRemote(tx); err != nil {
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
		watcherAction := proxy.DualBlockChain().DB().ReadEvent(contractAddress, msg.MethodName)
		if watcherAction != nil {

			// TODO(@kiendn, KSML): if watcherAction is matched then execute pre-defined code for this action
			//  currently, hardcode here for exchange case, will remove/move these after KSML is applied
			receiver := []byte(msg.GetParams()[0])
			to := msg.GetParams()[1]
			from := proxy.Name()

			txHash := common.HexToHash(msg.GetTransactionId())

			// Compose extraData struct for fields related to exchange from data extracted by Neo event
			extraData := make([][]byte, configs.ExchangeV2NumOfExchangeDataField)
			extraData[configs.ExchangeV2SourcePairIndex] = []byte(from)
			extraData[configs.ExchangeV2DestPairIndex] = []byte(to)
			extraData[configs.ExchangeV2SourceAddressIndex] = []byte(msg.GetSender())
			extraData[configs.ExchangeV2DestAddressIndex] = receiver
			extraData[configs.ExchangeV2OriginalTxIdIndex] = []byte(msg.GetTransactionId())
			extraData[configs.ExchangeV2AmountIndex] = big.NewInt(int64(msg.GetAmount())).Bytes()
			extraData[configs.ExchangeV2TimestampIndex] = big.NewInt(int64(msg.GetTimestamp())).Bytes()

			return NewEvent(proxy, msg.BlockNumber, msg.MethodName, big.NewInt(int64(msg.Amount)), extraData, txHash, watcherAction.DualAction, true)
		}
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
func NewEvent(proxy base.BlockChainAdapter, blockHeight uint64, method string, value *big.Int, extraData [][]byte, txHash common.Hash, action string, fromExternal bool) error {
	if proxy.DualBlockChain().Config().BaseAccount == nil {
		return fmt.Errorf("current node does not have base account to create new event")
	}

	baseAddress := proxy.DualBlockChain().Config().BaseAccount.Address
	privateKey := proxy.DualBlockChain().Config().BaseAccount.PrivateKey
	proxy.Logger().Info("nonce of base account", "baseAddress", baseAddress.Hex())

	eventSummary := &types.EventSummary{
		TxMethod: method,
		TxValue:  value,
		ExtData:  extraData,
	}

	if !AvailableExchangeType[proxy.Name()] {
		return fmt.Errorf("proxy %v is not in allowed exchanged list", proxy.Name())
	}

	dualEvent := types.NewDualEvent(blockHeight, fromExternal /* internalChain */, types.BlockchainSymbol(proxy.Name()), &txHash, eventSummary, &types.DualAction{
		Name: action,
	})

	// Compose extraData struct for fields related to exchange
	txMetaData, err := proxy.InternalChain().ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error compute internal tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetaData
	signedEvent, err := types.SignEvent(dualEvent, &privateKey)
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

// Release create release-assets event, txId is kardiaTxId which is used for callback method.
// create NewEvent here to make sure only proposer can submit event to target chain
func Release(proxy base.BlockChainAdapter, smc common.Address, kAbi abi.ABI, receiver, txId, amount string) error {

	if proxy.KardiaBlockChain().Config().BaseAccount == nil {
		return fmt.Errorf("BaseAccount is nil")
	}
	senderAddr := proxy.KardiaBlockChain().Config().BaseAccount.Address
	// get target chain contract address
	input, err := kAbi.Pack(configs.GetAddressFromType, proxy.Name())
	if err != nil {
		return err
	}
	result, err := CallStaticKardiaMasterSmc(senderAddr, smc, proxy.KardiaBlockChain(), input, proxy.KardiaTxPool().State())

	var smartContract string
	err = kAbi.Unpack(&smartContract, configs.GetAddressFromType, result)
	if err != nil {
		return err
	}

	// publish released data to zeroMQ
	// create a triggeredMessage and send it through ZeroMQ with topic KARDIA_CALL
	triggerMessage := dualMsg.TriggerMessage{
		ContractAddress: smartContract,
		MethodName:      configs.ExternalReleaseFunction,
		Params:          []string{receiver, amount},
		CallBacks: []*dualMsg.TriggerMessage{
			{
				ContractAddress: smc.Hex(),
				MethodName:      configs.UpdateTargetTx,
				Params: []string{
					// original tx, callback will be called after dual finish execute method,
					// txid after dual finish executing will be appended
					// callback method will be sent through 0MQ with DUAL_CALL topic
					txId,
				},
			},
		},
	}

	// Create KARDIA_CALL event
	proxy.Logger().Info("Publishing triggerMessage to event", "triggerMessage", triggerMessage.String())

	// Marshaling triggerMessage to byte array and put it to extraData
	//extraData := make([][]byte, 1)
	//buffer := &bytes.Buffer{}
	//marshaller := jsonpb.Marshaler{}
	//err = marshaller.Marshal(buffer, &triggerMessage)
	//if err != nil {
	//	return err
	//}
	//extraData[0] = buffer.Bytes()
	//txHash := common.HexToHash(txId)
	//blockHeight := proxy.DualBlockChain().CurrentBlock().Height()
	//return NewEvent(proxy, blockHeight, KARDIA_CALL, big.NewInt(0), extraData, txHash, KARDIA_CALL, false)
	return PublishMessage(proxy.PublishedEndpoint(), KARDIA_CALL, triggerMessage)
}

// KardiaCall receives event from submitTx and publish message to Target chain.
func KardiaCall(proxy base.BlockChainAdapter, event *types.EventData) error {

	// ExtData must have length = 1 and first element must not be nil
	if len(event.Data.ExtData) != 1 || event.Data.ExtData == nil {
		return fmt.Errorf("extData is invalid or empty in KardiaCall")
	}

	// unmarshal byte array data from ExtData
	unmarshaler := jsonpb.Unmarshaler{}
	reader := bytes.NewReader(event.Data.ExtData[0])
	triggerMessage := dualMsg.TriggerMessage{}
	err := unmarshaler.Unmarshal(reader, &triggerMessage)
	if err != nil {
		proxy.Logger().Error("Error while unmarshaling triggerMessage from EventData", "err", err)
		return err
	}

	return PublishMessage(proxy.PublishedEndpoint(), KARDIA_CALL, triggerMessage)
}

// getKardiaSmcAndAbiFromDual gets internal chain smart contract and abi from external chain dual action
// NOTE: dual action must be unique in KSML.
func getKardiaSmcAndAbiFromDual(proxy base.BlockChainAdapter, dualAction string) (string, *abi.ABI) {
	externalDb := proxy.DualBlockChain().DB()
	internalDb := proxy.KardiaBlockChain().DB()

	// get external smc from dual Action
	externalSmc, _ := externalDb.ReadSmartContractFromDualAction(dualAction)
	if externalSmc == "" {
		return "", nil
	}

	// get external watched event from externalSmc
	actions := externalDb.ReadEvents(externalSmc)
	if len(actions) == 0 {
		return "", nil
	}

	var smartContract string
	var kAbi *abi.ABI

	// find master smart contract
	for _, action := range actions {
		smartContract, kAbi = internalDb.ReadSmartContractFromDualAction(action.DualAction)
		if kAbi != nil {
			break
		}
	}
	return smartContract, kAbi
}

// HandleAddOrderFunction handles event.data.txMethod = AddOrderFunction.
// This function is used in all proxy in SubmitTx function.
// This is step before releasing coin to external chain (TRX, NEO, ETH).
func HandleAddOrderFunction(proxy base.BlockChainAdapter, event *types.EventData) error {
	if len(event.Data.ExtData) != configs.ExchangeV2NumOfExchangeDataField {
		return configs.ErrInsufficientExchangeData
	}
	if proxy.KardiaBlockChain().Config().BaseAccount == nil {
		return fmt.Errorf("BaseAccount is nil")
	}
	stateDB := proxy.KardiaTxPool().State()
	senderAddr := proxy.KardiaBlockChain().Config().BaseAccount.Address
	originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])
	fromType := string(event.Data.ExtData[configs.ExchangeV2SourcePairIndex])
	toType := string(event.Data.ExtData[configs.ExchangeV2DestPairIndex])

	// get kardia smc and abi from external dual action name
	smc, kAbi := getKardiaSmcAndAbiFromDual(proxy, event.Action.Name)
	if smc == "" || kAbi == nil {
		return fmt.Errorf("cannot find internal smc and abi for dualAction %v", event.Action.Name)
	}

	// encode smc
	smartContract := common.HexToAddress(smc)

	fromAmount, toAmount, err := CallGetRate(smartContract, *kAbi, fromType, toType, proxy.KardiaBlockChain(), stateDB)
	if err != nil {
		return err
	}

	// We get all releasable orders which are matched with newly added order
	releases, err := CallKardiGetMatchingResultByTxId(
		senderAddr,
		proxy.KardiaBlockChain(),
		stateDB,
		originalTx)
	if err != nil {
		return err
	}
	proxy.Logger().Info("Release info", "release", releases, "sender", senderAddr.Hex(), "originalTx", originalTx,
		"fromType", fromType, "toType", toType, "fromAmount", fromAmount, "toAmount", toAmount)

	if releases != "" {
		fields := strings.Split(releases, configs.ExchangeV2ReleaseFieldsSeparator)
		if len(fields) != 4 {
			proxy.Logger().Error("Invalid number of field", "release", releases)
			return errors.New("invalid number of field for release")
		}
		arrTypes := strings.Split(fields[configs.ExchangeV2ReleaseToTypeIndex], configs.ExchangeV2ReleaseValuesSepatator)
		arrAddresses := strings.Split(fields[configs.ExchangeV2ReleaseAddressesIndex], configs.ExchangeV2ReleaseValuesSepatator)
		arrAmounts := strings.Split(fields[configs.ExchangeV2ReleaseAmountsIndex], configs.ExchangeV2ReleaseValuesSepatator)
		arrTxIds := strings.Split(fields[configs.ExchangeV2ReleaseTxIdsIndex], configs.ExchangeV2ReleaseValuesSepatator)

		for i, t := range arrTypes {
			proxy.Logger().Error("start release", "type", t, "proxy", proxy.Name())
			if proxy.Name() != t {
				continue
			}

			if arrAmounts[i] == "" || arrAddresses[i] == "" || arrTxIds[i] == "" {
				proxy.Logger().Error("Missing release info", "matchedTxId", arrTxIds[i], "field", i, "releases", releases)
				continue
			}
			log.Info("Release info", "type", t, "address", arrAddresses[i], "amount", arrAmounts[i], "matchedTxId", arrTxIds[i])

			if t == configs.TRON || t == configs.NEO || t == configs.ETH {
				address := arrAddresses[i]
				amount, err1 := strconv.ParseInt(arrAmounts[i], 10, 64) //big.NewInt(0).SetString(arrAmounts[i], 10)
				proxy.Logger().Info("Amount from smc", "amount", amount, "in string", arrAmounts[i])
				if err1 != nil {
					log.Error("Error parse amount from smc", "amount", arrAmounts[i])
					continue
				}
				// Get rate base on the dual node exchange
				if t != fromType { //neo != eth
					tempFromAmount := fromAmount
					fromAmount = toAmount
					toAmount = tempFromAmount
				} else {
					fromType = toType
				}

				var (
					releasedAmount *big.Int
					err            error
				)

				switch t {
				case configs.TRON:
					// TRON is the smallest unit then do nothing with it
					releasedAmount = big.NewInt(amount)
				case configs.NEO:
					releasedAmount, err = CalculateReleasedAmountFromNeo(t, amount, fromAmount, toAmount, fromType)
				case configs.ETH:
					releasedAmount, err = CalculateReleasedAmountToEth(amount, fromAmount, toAmount, fromType)
				}

				if err != nil {
					proxy.Logger().Error(fmt.Sprintf("Error while calculating released amount from %v", t), "originalTxId", originalTx, "err", err, "amount", releasedAmount)
					return err
				}

				if err := Release(proxy, smartContract, *kAbi, address, arrTxIds[i], releasedAmount.String()); err != nil {
					proxy.Logger().Error("Error when releasing", "err", err.Error())
					return err
				}
			}
		}
	}
	log.Info("There is no matched result for tx", "originalTxId", originalTx)
	return nil
}

// CalculateReleasedAmountFromNeo calculates released amount from NEO to others chain
// NOTE: this func is only used for DEX case
func CalculateReleasedAmountFromNeo(releaseType string, amount int64, fromAmount, toAmount *big.Int, fromType string) (*big.Int, error) {
	var releasedAmount *big.Int
	if fromType == configs.ETH {
		// Divide amount from smart contract by 10^8 to get base NEO amount to release
		releasedAmount = big.NewInt(amount).Div(big.NewInt(amount), TenPoweredByEight)
	} else {
		// fromType is TRON
		// Calculate the releasedAmount based on the rate (fromAmount, toAmount)
		releaseByFloat := big.NewFloat(float64(amount))
		releaseByFloat = releaseByFloat.Mul(releaseByFloat, new(big.Float).SetInt(toAmount))
		releaseByFloat = releaseByFloat.Quo(releaseByFloat, new(big.Float).SetInt(fromAmount))
		// divide by 10^6 to get normal number
		releaseByFloat = releaseByFloat.Quo(releaseByFloat, TenPoweredBySixFloat)
		temp, _ := releaseByFloat.Float64()
		releasedAmount = big.NewInt(int64(math.Round(temp)))
	}
	// don't release  NEO if quantity < 1
	if releaseType == configs.NEO && releasedAmount.Cmp(big.NewInt(1)) < 0 {
		return nil, errAmountLessThanOne
	}
	return releasedAmount, nil
}

// CalculateReleasedAmountToEth calculates released amount from others dual node to ETH
// NOTE: this func is only used for DEX case
func CalculateReleasedAmountToEth(amount int64, fromAmount, toAmount *big.Int, fromType string) (*big.Int, error) {
	// Calculate the released amount by wei
	convertedAmount := big.NewFloat(float64(amount))
	convertedAmount = convertedAmount.Quo(convertedAmount, new(big.Float).SetInt(fromAmount))
	convertedAmount = convertedAmount.Mul(convertedAmount, new(big.Float).SetInt(toAmount))
	switch fromType {
	case configs.NEO:
		// if fromType is NEO then convert from NEO unit (10^8) to 10^18
		convertedAmount = big.NewFloat(float64(1)).Mul(convertedAmount, TenPoweredByTenFloat)
		temp, _ := convertedAmount.Float64()
		return big.NewInt(int64(math.Round(temp))), nil
	case configs.TRON:
		// if fromType is TRON then convert from TRON unit (10^6) to 10^18
		convertedAmount = big.NewFloat(float64(1)).Mul(convertedAmount, TenPoweredByTwelveFloat)
		temp, _ := convertedAmount.Float64()
		return big.NewInt(int64(math.Round(temp))), nil
	default:
		return nil, fmt.Errorf("invalid fromType %v", fromType)
	}
}
