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

package kardia

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"encoding/hex"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/base"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
)
// TODO(@sontranrad): remove all of these constants for production
const ETH2NEO = "ETH-NEO"
const NEO2ETH = "NEO-ETH"
const KardiaAccountToCallSmc = "0xBA30505351c17F4c818d94a990eDeD95e166474b"
const KardiaPrivKeyToCallSmc = "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7"
const KardiaNewExchangeSmcIndex = 3

// These const contain indices of exchange-related fields in ExtData field of EventSummary struct
const ExchangeDataSourceAddressIndex = 0
const ExchangeDataDestAddressIndex = 1
const ExchangeDataSourcePairIndex = 2
const ExchangeDataDestPairIndex = 3
const ExchangeDataAmountIndex = 4
const ExchangeDataCompleteRequestIDIndex = 0
const ExchangeDataCompletePairIndex = 1
const NumOfExchangeDataField = 5
const NumOfCompleteRequestDataField = 2
var MaximumGasToCallStaticFunction = uint(4000000)
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
func CallGetRate(pair string, bc base.BaseBlockChain, statedb *state.StateDB) (sale *big.Int, receive *big.Int, err error) {
	senderAddr := common.HexToAddress(configs.MockSmartContractCallSenderAccount)
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		return big.NewInt(0), big.NewInt(0), err
	}
	getRateInput, err := kAbi.Pack("getRatePublic", pair)
	if err != nil {
		log.Error("Error packing get rate input", "err", err)
		return big.NewInt(0), big.NewInt(0), err
	}
	result, err := CallStaticKardiaMasterSmc(senderAddr, masterSmcAddr, bc, getRateInput, statedb);
	if  err != nil {
		log.Error("Error call get rate", "err", err)
		return big.NewInt(0), big.NewInt(0), err
	}
	var rateStruct struct {
		Sale    *big.Int `abi:"sale"`
		Receive *big.Int `abi:"receive"`
	}
	err = kAbi.Unpack(&rateStruct, "getRatePublic", result)
	if err != nil {
		log.Error("Error unpack rate result", "err", err)
		return big.NewInt(0), big.NewInt(0), err
	}
	return rateStruct.Sale, rateStruct.Receive, nil
}

// CallAvailableAmount get available amount to exchange for a pair
func CallAvailableAmount(pair string, bc base.BaseBlockChain, statedb *state.StateDB) (amount *big.Int, err error) {
	senderAddr := common.HexToAddress(configs.MockSmartContractCallSenderAccount)
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
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
			destinationAddress string, source types.BlockchainSymbol) (*types.Transaction, error) {
	// Change master smc index to 3 for the new exchange contract
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	var matchInput []byte
	switch source {
	case types.ETHEREUM:
		matchInput, err = kABI.Pack("matchRequest", ETH2NEO, NEO2ETH, sourceAddress, destinationAddress, quantity)
	case types.NEO:
		matchInput, err = kABI.Pack("matchRequest", NEO2ETH, ETH2NEO, sourceAddress, destinationAddress, quantity)
	default:
		log.Error("Invalid source for match amount tx", "src", source)
		return nil, err
	}
	log.Info("Matching", "quantity", quantity, "source", source, "source", sourceAddress,
		"dest", destinationAddress)
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, matchInput, statedb), nil
}

// Call to get a matched request of a newly added request of exchange contract. This function will return a MatchedRequest
// contains MatchedRequestID, Address, SendAmount or nil in case of error
func CallKardiaGetMatchedRequest(from common.Address, bc base.BaseBlockChain, statedb *state.StateDB,
		quantity *big.Int, sourceAddress string, destinationAddress string, sourcePair string, interestedPair string) (*MatchedRequest, error) {
	// Change master smc index to 3 for the new exchange contract
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	var getMatchedRequestInput []byte
	// getMatchingInput1, e5 = abi.Pack("getMatchingRequestInfo", "ETH-NEO", "ethsender1", "neoReceiver1", big.NewInt(1))
	log.Info("get matching:", "pair", sourcePair, "src", sourceAddress, "dest", destinationAddress, "quan", quantity)
	getMatchedRequestInput, err = kABI.Pack("getMatchingRequestInfo", sourcePair, interestedPair, sourceAddress, destinationAddress, quantity);
	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr)
		return nil, err
	}
	getMatchedRequestResult, err := CallStaticKardiaMasterSmc(from, masterSmcAddr, bc, getMatchedRequestInput, statedb);
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

// CreateKardiaCompleteRequestTx creates a tx to Kardia exchange smc to complete an exchange request, params sent to smc
// are requestID (stored in smc) and pair
func CreateKardiaCompleteRequestTx(state *state.ManagedState, requestID *big.Int, pair string) (*types.Transaction, error) {
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
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

// CallKardiaGetUncompletedRequest calls to Kardia exchange smc to get a matching but uncompleted request of a specific request
func CallKardiaGetUncompletedRequest(requestID *big.Int, stateDb *state.StateDB, abi *abi.ABI, bc base.BaseBlockChain) (*MatchedRequest, error) {
	getUncompletedInput, err := abi.Pack("getUncompletedMatchingRequest", requestID)
	if err != nil {
		log.Error("Error packing input for getUncompletedMatchingRequest", "err", err)
		return nil, err
	}
	senderAddr := common.HexToAddress(configs.MockSmartContractCallSenderAccount)
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
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
func CreateKardiaSetRateTx(pair string, sale *big.Int, receive *big.Int, state *state.ManagedState) (*types.Transaction, error) {
	log.Info("Setting rate", "pair", pair)
	masterSmcAddr := configs.GetContractAddressAt(KardiaNewExchangeSmcIndex)
	masterSmcAbi := configs.GetContractAbiByAddress(masterSmcAddr.String())
	kAbi, err := abi.JSON(strings.NewReader(masterSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	setRateInput, err := kAbi.Pack("addRate", pair, sale, receive)
	if  err != nil {
		log.Error("Failed to pack set Rate input", "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(GetPrivateKeyToCallKardiaSmc(), masterSmcAddr, setRateInput, state), nil
}

// Return a common private key to call to Kardia smc from dual node
func GetPrivateKeyToCallKardiaSmc() *ecdsa.PrivateKey {
	addrKeyBytes, _ := hex.DecodeString(KardiaPrivKeyToCallSmc)
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	return addrKey
}

