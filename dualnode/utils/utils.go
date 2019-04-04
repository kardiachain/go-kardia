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
	"crypto/ecdsa"
	"encoding/hex"
	"github.com/kardiachain/go-kardia/configs"
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
	"github.com/pkg/errors"
	"math/big"
	"strings"
	"time"
	"github.com/pebbe/zmq4"
	"fmt"
)

// TODO(@sontranrad): remove all of these constants for production

const KardiaAccountToCallSmc = "0xBA30505351c17F4c818d94a990eDeD95e166474b"
const KardiaPrivKeyToCallSmc = "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7"

// TODO: note that when we have dynamic method, these values will be moved to smartcontract or anything that can handle this case.
var AvailableExchangeType = map[string]bool{
	configs.TRON: true,
	configs.NEO: true,
	configs.ETH: true,
}

var MaximumGasToCallStaticFunction = uint(4000000)
var errAbiNotFound = errors.New("ABI not found")

var TenPoweredByEight = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(8), nil)
var TenPoweredBySix = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(6), nil)
var TenPoweredByTen = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(10), nil)
var OneEthInWei = big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil)

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

// CallGetRate calls to Kardia exchange contract to get rate of a specific pair, return sale amount and receive amount
func CallGetRate(fromType string, toType string, bc base.BaseBlockChain, statedb *state.StateDB) (fromAmount *big.Int, receivedAmount *big.Int, err error) {
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))

	senderAddr := common.HexToAddress(configs.MockSmartContractCallSenderAccount)
	getRateInput, err := kABI.Pack("getRate", fromType, toType)
	if err != nil {
		log.Error("Error packing get rate input", "err", err)
		if fromType == configs.NEO {
			return big.NewInt(configs.RateNEO), big.NewInt(configs.RateETH), err
		}
		return big.NewInt(configs.RateETH), big.NewInt(configs.RateNEO), err
	}
	result, err := CallStaticKardiaMasterSmc(senderAddr, masterSmcAddr, bc, getRateInput, statedb)
	if err != nil {
		log.Error("Error call get rate", "err", err)
		if fromType == configs.NEO {
			return big.NewInt(configs.RateNEO), big.NewInt(configs.RateETH), err
		}
		return big.NewInt(configs.RateETH), big.NewInt(configs.RateNEO), err
	}
	var rateStruct struct {
		FromAmount 		*big.Int
		ReceivedAmount 	*big.Int
	}
	err = kABI.Unpack(&rateStruct, "getRate", result)
	if err != nil {
		log.Error("Error unpack rate result", "err", err)
		if fromType == configs.NEO {
			return big.NewInt(configs.RateNEO), big.NewInt(configs.RateETH), err
		}
		return big.NewInt(configs.RateETH), big.NewInt(configs.RateNEO), err
	}
	return rateStruct.FromAmount, rateStruct.ReceivedAmount, nil
}

// CallAvailableAmount gets available amount to exchange for a pair
func CallAvailableAmount(pair string, bc base.BaseBlockChain, statedb *state.StateDB) (amount *big.Int, err error) {
	senderAddr := common.HexToAddress(configs.MockSmartContractCallSenderAccount)
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		return big.NewInt(0), err
	}
	getAvailableInput, err := abi.Pack("getAvailableAmountByPair", pair)
	if err != nil {
		log.Error("Error packing get available amount", "err", err)
		return big.NewInt(0), err
	}
	availableResult, err := CallStaticKardiaMasterSmc(senderAddr, masterSmcAddr, bc, getAvailableInput, statedb)
	if err != nil {
		log.Error("Error get available amount", "err", err)
		return big.NewInt(0), err
	}
	return big.NewInt(0).SetBytes(availableResult), nil
}

// Creates a Kardia tx to report new matching amount from Eth/Neo network, return nil in case of any error occurs
func CreateKardiaMatchAmountTx(statedb *state.ManagedState, quantity *big.Int, sourceAddress string,
	destinationAddress string, source types.BlockchainSymbol, hash string, bc base.BaseBlockChain) (*types.Transaction, error) {
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
	rateEth, rateNeo, err := CallGetRate(configs.ETH, configs.NEO, bc, statedb.StateDB)
	// return default if err
	if err != nil {
		rateNeo = big.NewInt(configs.RateNEO)
		rateEth = big.NewInt(configs.RateETH)
	}
	switch source {
	case types.ETHEREUM:
		convertedAmount := temp.Mul(quantity, rateEth)
		convertedAmount = temp.Div(convertedAmount, rateNeo)
		convertedAmount = temp.Div(convertedAmount, TenPoweredByTen)
		log.Info("AddOrderFunction", "fromType", configs.ETH, "toType", configs.NEO, "srcAddress", sourceAddress,
			"destAddress", destinationAddress, "originalTx", hash, "quantity", convertedAmount.String(), "timestamp", timestamp)
		matchInput, err = kABI.Pack(configs.AddOrderFunction, configs.ETH, configs.NEO, sourceAddress,
			destinationAddress, hash, convertedAmount, timestamp)
	case types.NEO:
		// Multiply original amount by 10^8
		releasedAmount := temp.Mul(quantity, TenPoweredByEight)
		log.Info("AddOrderFunction", "fromType", configs.NEO, "toType", configs.ETH, "srcAddress", sourceAddress,
			"destAddress", destinationAddress, "originalTx", hash, "quantity", quantity.String(), "timestamp", timestamp)
		matchInput, err = kABI.Pack(configs.AddOrderFunction, configs.NEO, configs.ETH, sourceAddress,
			destinationAddress, hash, releasedAmount, timestamp)
	default:
		log.Error("Invalid source for match amount tx", "src", source)
		return nil, err
	}
	log.Info("Adding order and matching: ", "txhash", hash, "quantity", quantity, "source", source, "source", sourceAddress, "dest", destinationAddress)
	if err != nil {
		log.Error("Error packing abi", "error", err, "address")
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, matchInput, statedb), nil
}

// Call to get a matched request of a newly added request of exchange contract. This function will return a MatchedRequest
// contains MatchedRequestID, Address, SendAmount or nil in case of error
func CallKardiaGetMatchedRequest(from common.Address, bc base.BaseBlockChain, statedb *state.StateDB,
	quantity *big.Int, sourceAddress string, destinationAddress string, sourcePair string, interestedPair string) (*MatchedRequest, error) {
	// Change master smc index to 3 for the new exchange contract
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	var getMatchedRequestInput []byte
	// getMatchingInput1, e5 = abi.Pack("getMatchingRequestInfo", "ETH-NEO", "ethsender1", "neoReceiver1", big.NewInt(1))
	log.Info("get matching:", "pair", sourcePair, "src", sourceAddress, "dest", destinationAddress, "quan", quantity)
	getMatchedRequestInput, err = kABI.Pack("getMatchingRequestInfo", sourcePair, interestedPair, sourceAddress, destinationAddress, quantity)
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return nil, err
	}
	getMatchedRequestResult, err := CallStaticKardiaMasterSmc(from, masterSmcAddr, bc, getMatchedRequestInput, statedb)
	if err != nil {
		log.Error("Error get match request", "err", err)
		return nil, err
	}
	var request MatchedRequest
	err = kABI.Unpack(&request, "getMatchingRequestInfo", getMatchedRequestResult)
	if err != nil {
		log.Error("Error unpack getMatchingRequestInfo", "err", err)
		return nil, err
	}
	return &request, nil
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

func CallKardiaGetReleaseByTxId(from common.Address, bc base.BaseBlockChain, statedb *state.StateDB,
	orginalTx string, interestedPair string) (string, error) {
	// Change master smc index to 3 for the new exchange contract
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return "", err
	}
	var getReleaseByTxId []byte
	// getMatchingInput1, e5 = abi.Pack("getMatchingRequestInfo", "ETH-NEO", "ethsender1", "neoReceiver1", big.NewInt(1))
	log.Info("CallKardiaGetReleaseByTxId", "originalTx", orginalTx, "pair", interestedPair)
	getReleaseByTxId, err = kABI.Pack("getReleaseByTxId", orginalTx, interestedPair)
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return "", err
	}
	getReleaseByTxIdResult, err := CallStaticKardiaMasterSmc(from, masterSmcAddr, bc, getReleaseByTxId, statedb)
	if err != nil {
		log.Error("Error getReleaseByTxId", "err", err)
		return "", err
	}
	var releases struct {
		ReleaseInfos string
	}
	err = kABI.Unpack(&releases, "getReleaseByTxId", getReleaseByTxIdResult)
	if err != nil {
		log.Error("Error unpack getReleaseByTxId", "err", err)
		return "", err
	}
	return releases.ReleaseInfos, nil
}

// CreateKardiaCompleteRequestTx creates a tx to Kardia exchange smc to complete an exchange request, params sent to smc
// are requestID (stored in smc) and pair
func CreateKardiaCompleteRequestTx(state *state.ManagedState, requestID *big.Int, pair string) (*types.Transaction, error) {
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	completeInput, err := kAbi.Pack("completeRequest", requestID, pair)
	if err != nil {
		log.Error("Failed to pack complete request input", "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, completeInput, state), nil
}

func CreateKardiaCompleteOrder(state *state.ManagedState, originalTx string, releaseTx string, pair string, receiver string, amount *big.Int) (*types.Transaction, error) {
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	completeInput, err := kAbi.Pack("completeOrder", originalTx, releaseTx, pair, receiver, amount)
	if err != nil {
		log.Error("Failed to pack completeOrder input", "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, completeInput, state), nil
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
		completeInput, err = kAbi.Pack("updateTargetTx", originalTx, tx)
	} else {
		completeInput, err = kAbi.Pack("updateKardiaTx", originalTx, tx)
	}
	if err != nil {
		log.Error("Failed to pack updateTx", txType, "originalTx", originalTx, "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, completeInput, state), nil
}

// CallKardiaGetUncompletedRequest calls to Kardia exchange smc to get a matching but uncompleted request of a specific request
func CallKardiaGetUncompletedRequest(requestID *big.Int, stateDb *state.StateDB, abi *abi.ABI, bc base.BaseBlockChain) (*MatchedRequest, error) {
	getUncompletedInput, err := abi.Pack("getUncompletedMatchingRequest", requestID)
	if err != nil {
		log.Error("Error packing input for getUncompletedMatchingRequest", "err", err)
		return nil, err
	}
	senderAddr := common.HexToAddress(configs.MockSmartContractCallSenderAccount)
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	getUncompletedResult, err := CallStaticKardiaMasterSmc(senderAddr, masterSmcAddr, bc, getUncompletedInput, stateDb)
	if err != nil {
		log.Error("Cannot get uncomplete matching request", "err", err)
		return nil, err
	}
	var request MatchedRequest
	err = abi.Unpack(&request, "getUncompletedMatchingRequest", getUncompletedResult)
	if err != nil {
		log.Error("Cannot unpack result from getUncompletedMatching request", "err", err)
		return nil, err
	}
	return &request, nil
}

// CreateKardiaSetRateTx creates tx call to Kardia exchange smart contract to update radte of a specific pair
// If 1 ETH = 10 NEO, call with pairs ("ETH-NEO", 1, 10) and ("NEO-ETH", 10,1)
func CreateKardiaSetRateTx(fromPair string, toPair string, sale *big.Int, receive *big.Int, state *state.ManagedState) (*types.Transaction, error) {
	log.Info("Setting rate", "Rate", fromPair, "toPair" , toPair)
	masterSmcAddr := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	setRateInput, err := kAbi.Pack("updateRate", fromPair, toPair, sale, receive)
	if err != nil {
		log.Error("Failed to pack update Rate input", "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, setRateInput, state), nil
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
	addrKeyBytes, _ := hex.DecodeString(KardiaPrivKeyToCallSmc)
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	return addrKey
}

func IsNilOrEmpty(data []byte) bool { return data == nil || string(data) == "" }

func PublishMessage(endpoint, topic, message string) error {
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
	if _, err := pub.Send(message, zmq4.DONTWAIT); err != nil {
		return err
	}

	return nil
}

// GetExchangePair split string into 2 pairs and validate if 2 pairs are valid or not.
func GetExchangePair(pair string) (*string, *string, error) {
	pairs := strings.Split(pair, "-")
	if len(pairs) != 2 {
		return nil, nil, fmt.Errorf("invalid pair %v", pairs)
	}

	if _, ok := AvailableExchangeType[pairs[0]]; !ok {
		return nil, nil, fmt.Errorf("invalid first type %v", pairs[0])
	}

	if _, ok := AvailableExchangeType[pairs[1]]; !ok {
		return nil, nil, fmt.Errorf("invalid second type %v", pairs[1])
	}

	return &pairs[0], &pairs[1], nil
}

// ExecuteKardiaSmartContract executes
func ExecuteKardiaSmartContract(state *state.ManagedState, contractAddress, methodName string, params []string) (*types.Transaction, error) {
	masterSmcAddr := common.HexToAddress(contractAddress)
	// TODO: replace this line to function that get abi from contractAddress
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
