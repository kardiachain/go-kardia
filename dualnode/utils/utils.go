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
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pebbe/zmq4"
	"github.com/pkg/errors"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode"
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
	DUAL_CALL = "DUAL_CALL"
	DUAL_MSG = "DUAL_MSG"
)

// TODO: note that when we have dynamic method, these values will be moved to smartcontract or anything that can handle this case.
var AvailableExchangeType = map[string]bool{
	configs.TRON: true,
	configs.NEO: true,
	configs.ETH: true,
}

var MaximumGasToCallStaticFunction = uint(4000000)
var errAbiNotFound = errors.New("ABI not found")
var errNoNeoToSend        = errors.New("not enough NEO to send")
var TenPoweredBySix = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(6), nil)
var TenPoweredByEight = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(8), nil)
var TenPoweredByTen = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(10), nil)
var TenPoweredByTwelve = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(12), nil)
var TenPoweredBySixFloat =  big.NewFloat(float64(math.Pow10(6)))

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
func CallGetRate(fromType string, toType string, bc base.BaseBlockChain, statedb *state.StateDB) (fromAmount *big.Int, receivedAmount *big.Int, err error) {
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))

	senderAddr := common.HexToAddress(configs.KardiaAccountToCallSmc)
	getRateInput, err := kABI.Pack("getRate", fromType, toType)
	if err != nil {
		log.Error("Error packing get rate input", "err", err)
		// get default rate if error is thrown
		return configs.GetRateFromType(fromType), configs.GetRateFromType(toType), err
	}

	result, err := CallStaticKardiaMasterSmc(senderAddr, masterSmcAddr, bc, getRateInput, statedb)
	if err != nil {
		log.Error("Error call get rate", "err", err)
		// get default rate if error is thrown
		return configs.GetRateFromType(fromType), configs.GetRateFromType(toType), err
	}

	// init a rateStruct based on returned type from smart contract
	var rateStruct struct {
		FromAmount 		*big.Int
		ReceivedAmount 	*big.Int
	}
	err = kABI.Unpack(&rateStruct, "getRate", result)
	if err != nil {
		log.Error("Error unpack rate result", "err", err)
		// get default rate if error is thrown
		return configs.GetRateFromType(fromType), configs.GetRateFromType(toType), err
	}
	return rateStruct.FromAmount, rateStruct.ReceivedAmount, nil
}

// Creates a Kardia tx to report new matching amount from Eth/Neo/TRX network, return nil in case of any error occurs
func CreateKardiaMatchAmountTx(statedb *state.ManagedState, quantity *big.Int, sourceAddress string,
	destinationAddress string, source, destination string, hash string, bc base.BaseBlockChain) (*types.Transaction, error) {

	// check if source and destination types are valid or not.
	if !AvailableExchangeType[source] || !AvailableExchangeType[destination] {
		return nil, fmt.Errorf("invalid type")
	}

	// Change master smc index to 3 for the new exchange contract
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	var matchInput []byte
	timestamp := big.NewInt(time.Now().Unix())
	temp := big.NewInt(1)
	fromAmount, toAmount, err := CallGetRate(source, destination, bc, statedb.StateDB)
	if err != nil {
		return nil, err
	}

	var convertedAmount *big.Int

	// unit of ordered amount will be based on the type which has smaller unit based.
	// for eg: int ETH-NEO, NEO has 10^8 while ETH has 10^18, hence the order amount will be based on NEO
	log.Info("Prepare for convert amount", "source", source, "destination", destination,
		"fromAmount", fromAmount, "toAmount", toAmount)
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
			convertedAmount = temp.Mul(quantity, TenPoweredBySix)
			convertedAmount = temp.Mul(convertedAmount, fromAmount)
			convertedAmount = temp.Div(convertedAmount, toAmount)
		}
	case configs.TRON:
		// currently TRON has smallest unit, therefore no need to calculate anything here.
		convertedAmount = quantity
	default:
		log.Error("Invalid source for match amount tx", "src", source)
		return nil, err
	}

	log.Info("AddOrderFunction", "fromType", source, "toType", destination, "srcAddress", sourceAddress,
		"destAddress", destinationAddress, "originalTx", hash, "quantity", convertedAmount.String(), "timestamp", timestamp)

	matchInput, err = kABI.Pack(configs.AddOrderFunction, source, destination, sourceAddress,
		destinationAddress, hash, convertedAmount, timestamp)
	if err != nil {
		log.Error("Error packing abi", "error", err, "address")
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, matchInput, statedb), nil
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

func UpdateKardiaTargetTx(state *state.ManagedState, originalTx string, tx string, txType string) (*types.Transaction, error) {
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	var completeInput []byte
	if txType == "target" {
		completeInput, err = kAbi.Pack(configs.UpdateTargetTx, originalTx, tx)
	} else {
		completeInput, err = kAbi.Pack(configs.UpdateKardiaTx, originalTx, tx)
	}
	if err != nil {
		log.Error("Failed to pack updateTx", txType, "originalTx", originalTx, "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, completeInput, state), nil
}

// CreateForwardRequestTx creates tx call to Kardia candidate exchange contract to forward a candidate request to another
// external chain
func CreateForwardRequestTx(email string, fromOrgId string, toOrgId string, state *state.ManagedState) (*types.Transaction, error) {
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
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), exchangeSmcAddr, requestInfoInput, state), nil
}

// CreateForwardResponseTx creates tx call to Kardia candidate exchange contract to fulfill a candidate info request
// from external private chain, receiving private chain will catch the event fired from Kardia exchange contract to process
// candidate info
func CreateForwardResponseTx(email string, response string, fromOrgId string, toOrgId string,
	state *state.ManagedState) (*types.Transaction, error) {
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
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), exchangeSmcAddr, requestInfoInput, state), nil
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

	// send message
	log.Info("PublishMessage", "topic", topic, "msg", message.String() )
	if _, err := pub.Send(message.String(), zmq4.DONTWAIT); err != nil {
		return err
	}

	return nil
}

// ExecuteKardiaSmartContract executes smart contract based on address, method and list of params
func ExecuteKardiaSmartContract(state *state.ManagedState, contractAddress, methodName string, params []string) (*types.Transaction, error) {
	masterSmcAddr := common.HexToAddress(contractAddress)
	// TODO(@kiendn): replace this line to function that get abi from contractAddress
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
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
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, input, state), nil
}

// MessageHandler handles messages come from dual to kardia
func MessageHandler(proxy base.BlockChainAdapter, topic, message string) error {
	switch topic {
	case DUAL_CALL:
		// callback from dual
		triggerMessage := dualMsg.TriggerMessage{}
		if err := jsonpb.UnmarshalString(message, &triggerMessage); err != nil {
			return err
		}

		tx, err := ExecuteKardiaSmartContract(proxy.KardiaTxPool().State(), triggerMessage.ContractAddress, triggerMessage.MethodName, triggerMessage.Params)
		if err != nil {
			return err
		}

		if err := proxy.KardiaTxPool().AddLocal(tx); err != nil {
			return nil
		}

	case DUAL_MSG:
		// message from dual after it catches a triggered smc tx
		// unpack contents to DualMessage
		msg := dualMsg.Message{}
		if err := jsonpb.UnmarshalString(message, &msg); err != nil {
			return err
		}

		// TODO: this is used for exchange demo, remove the condition whenever we have dynamic handler method for this
		if msg.MethodName == configs.ExternalDepositFunction {
			return NewEvent(proxy, msg)
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

// NewEvent receives data from Tron where encodedMsg is used for validating the message
// returns error in case event cannot be added to eventPool
func NewEvent(proxy base.BlockChainAdapter, msg dualMsg.Message) error {
	dualState, err := proxy.DualBlockChain().State()
	if err != nil {
		log.Error("Fail to get Dual BlockChain state", "error", err)
		return err
	}

	// TODO: This is used for exchange use case, will remove this after applying dynamic method
	receiver := []byte(msg.GetParams()[0])
	to := msg.GetParams()[1]
	from := proxy.Name()

	txHash := common.HexToHash(msg.GetTransactionId())
	nonce := dualState.GetNonce(common.HexToAddress(event_pool.DualStateAddressHex))
	// Compose extraData struct for fields related to exchange from data extracted by Neo event
	extraData := make([][]byte, configs.ExchangeV2NumOfExchangeDataField)
	extraData[configs.ExchangeV2SourcePairIndex] = []byte(from)
	extraData[configs.ExchangeV2DestPairIndex] = []byte(to)
	extraData[configs.ExchangeV2SourceAddressIndex] = []byte(msg.GetSender())
	extraData[configs.ExchangeV2DestAddressIndex] = receiver
	extraData[configs.ExchangeV2OriginalTxIdIndex] = []byte(msg.GetTransactionId())
	extraData[configs.ExchangeV2AmountIndex] = big.NewInt(int64(msg.GetAmount())).Bytes()
	extraData[configs.ExchangeV2TimestampIndex] = big.NewInt(int64(msg.GetTimestamp())).Bytes()

	eventSummary := &types.EventSummary{
		TxMethod: msg.MethodName,
		TxValue:  big.NewInt(int64(msg.Amount)),
		ExtData:  extraData,
	}

	actionsTmp := [...]*types.DualAction{
		&types.DualAction{
			Name: dualnode.CreateKardiaMatchAmountTx,
		},
	}

	if !AvailableExchangeType[proxy.Name()] {
		return fmt.Errorf("proxy %v is not in allowed exchanged list", proxy.Name())
	}

	dualEvent := types.NewDualEvent(nonce, true /* internalChain */, types.BlockchainSymbol(proxy.Name()), &txHash, eventSummary, &types.DualActions{
		Actions: actionsTmp[:],
	})

	// Compose extraData struct for fields related to exchange
	txMetaData, err := proxy.InternalChain().ComputeTxMetadata(dualEvent.TriggeredEvent)
	if err != nil {
		log.Error("Error compute internal tx metadata", "err", err)
		return err
	}
	dualEvent.PendingTxMetadata = txMetaData
	err = proxy.DualEventPool().AddEvent(dualEvent)
	if err != nil {
		log.Error("Failed to add dual event to pool", "err", err)
		return err
	}
	log.Info("Added to dual event pool successfully", "eventHash", dualEvent.Hash().String())
	return nil
}

// Release releases assets to target chain to receiver, txId is kardiaTxId which is used for callback method.
func Release(proxy base.BlockChainAdapter, receiver, txId, amount string) error {
	senderAddr := common.HexToAddress(configs.KardiaAccountToCallSmc)
	exchangeSmcAddr, exchangeSmcAbi := configs.GetContractDetailsByIndex(configs.KardiaNewExchangeSmcIndex)
	if exchangeSmcAbi == "" {
		return errAbiNotFound
	}
	kAbi, err := abi.JSON(strings.NewReader(exchangeSmcAbi))
	if err != nil {
		return err
	}
	// get target chain contract address
	input, err := kAbi.Pack(configs.GetAddressFromType, proxy.Name())
	if err != nil {
		return err
	}

	result, err := CallStaticKardiaMasterSmc(senderAddr, exchangeSmcAddr, proxy.KardiaBlockChain(), input, proxy.KardiaTxPool().State().StateDB)

	var smartContract string
	err = kAbi.Unpack(&smartContract, configs.GetAddressFromType, result)
	if err != nil {
		return err
	}

	// publish released data to zeroMQ
	// create a triggeredMessage and send it through ZeroMQ with topic KARDIA_CALL
	triggerMessage := dualMsg.TriggerMessage{
		ContractAddress: smartContract,
		MethodName: configs.ExternalReleaseFunction,
		Params: []string{receiver, amount},
		CallBacks: []*dualMsg.TriggerMessage{
			{
				ContractAddress: configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex).Hex(),
				MethodName: configs.UpdateTargetTx,
				Params: []string{
					// original tx, callback will be called after dual finish execute method,
					// txid after dual finish executing will be appended
					// callback method will be sent through 0MQ with DUAL_CALL topic
					txId,
				},
			},
		},
	}
	return PublishMessage(proxy.PublishedEndpoint(), KARDIA_CALL, triggerMessage)
}

// HandleAddOrderFunction handles event.data.txMethod = AddOrderFunction.
// This function is used in all proxy in SubmitTx function.
// This is step before releasing coin to target chain (TRX, NEO).
func HandleAddOrderFunction(proxy base.BlockChainAdapter, event *types.EventData) error {
	if len(event.Data.ExtData) != configs.ExchangeV2NumOfExchangeDataField {
		return configs.ErrInsufficientExchangeData
	}
	stateDB := proxy.KardiaTxPool().State().StateDB
	senderAddr := common.HexToAddress(configs.KardiaAccountToCallSmc)
	originalTx := string(event.Data.ExtData[configs.ExchangeV2OriginalTxIdIndex])
	fromType := string(event.Data.ExtData[configs.ExchangeV2SourcePairIndex])
	toType := string(event.Data.ExtData[configs.ExchangeV2DestPairIndex])

	fromAmount, toAmount, err := CallGetRate(fromType, toType, proxy.KardiaBlockChain(), stateDB)
	if err != nil {
		return err
	}

	// We get all releasable orders which are matched with newly added order
	releases, err := CallKardiGetMatchingResultByTxId(
		senderAddr,
		proxy.KardiaBlockChain(),
		stateDB,
		originalTx,)
	if err != nil {
		return err
	}
	proxy.Logger().Info("Release info", "release", releases)
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

			if proxy.Name() != t {
				continue
			}

			if arrAmounts[i] == "" || arrAddresses[i] == "" || arrTxIds[i] == "" {
				proxy.Logger().Error("Missing release info", "matchedTxId", arrTxIds[i], "field", i, "releases", releases)
				continue
			}
			log.Info("ReleaseInfo", "type", t, "address", arrAddresses[i], "amount" , arrAmounts[i], "matchedTxId", arrTxIds[i])

			if t == configs.TRON || t == configs.NEO {
				address := arrAddresses[i]
				amount, err1 := strconv.ParseInt(arrAmounts[i], 10, 64) //big.NewInt(0).SetString(arrAmounts[i], 10)
				proxy.Logger().Info("Amount", "amount", amount, "in string", arrAmounts[i])
				if err1 != nil {
					log.Error("Error parse amount", "amount", arrAmounts[i])
					continue
				}

				var releasedAmount *big.Int
				if t == configs.TRON {
					// TRON is the smallest unit then do nothing with it
					releasedAmount = big.NewInt(amount)
				} else {
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
					proxy.Logger().Info("Prepare to release", "amount", releasedAmount)
					// don't release  NEO if quantity < 1
					if releasedAmount.Cmp(big.NewInt(1)) < 0 {
						proxy.Logger().Error("Too little neo to send", "originalTxId", originalTx, "err", errNoNeoToSend, "amount", releasedAmount)
						return errNoNeoToSend
					}

				}
				if err := Release(proxy, address, releasedAmount.String(), arrTxIds[i]); err != nil {
					proxy.Logger().Error("Error when releasing", "err", err.Error())
					return err
				}
			}
		}
	}
	log.Info("There is no matched result for tx", "originalTxId", originalTx)
	return nil
}
