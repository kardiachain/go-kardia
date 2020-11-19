/*
 *  Copyright 2019 KardiaChain
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

package tests

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kardiachain/go-kardiamain/configs"
	message2 "github.com/kardiachain/go-kardiamain/dualnode/message"
	"github.com/kardiachain/go-kardiamain/kai/blockchain"
	"github.com/kardiachain/go-kardiamain/kai/genesis"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/staking"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	"github.com/kardiachain/go-kardiamain/kai/tx_pool"
	"github.com/kardiachain/go-kardiamain/ksml"
	message "github.com/kardiachain/go-kardiamain/ksml/proto"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
	kaiType "github.com/kardiachain/go-kardiamain/types"
)

type MemoryDbInfo struct{}

func NewMemoryDbInfo() *MemoryDbInfo {
	return &MemoryDbInfo{}
}

func (db *MemoryDbInfo) Name() string {
	return "Memory"
}

func (db *MemoryDbInfo) Start() (types.StoreDB, error) {
	return kvstore.NewStoreDB(memorydb.New()), nil
}

func TestGetPrefix_WithoutPrefix(t *testing.T) {
	parser := ksml.Parser{}
	content := "1"
	prefix, method, params, err := parser.GetPrefix(content)
	require.NoError(t, err)
	require.Len(t, prefix, 0)
	require.Equal(t, method, content)
	require.Nil(t, params)
}

func TestGetPrefix_WithValidPrefix(t *testing.T) {
	parser := ksml.Parser{}
	content := "fn:currentTimeStamp()"
	prefix, method, params, err := parser.GetPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, "fn")
	require.Equal(t, method, "currentTimeStamp")
	require.Equal(t, len(params), 0)
}

func TestGetPrefix_WithParam(t *testing.T) {
	parser := ksml.Parser{}
	content := "smc:getData(getParams)"
	expectedMethod := "getData"
	expectedParams := []string{"getParams"}
	prefix, method, params, err := parser.GetPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, "smc")
	require.Equal(t, method, expectedMethod)
	require.Equal(t, params, expectedParams)
}

func TestGetPrefix_WithParams(t *testing.T) {
	parser := &ksml.Parser{}
	content := "smc:getData(getParams, message.sender, message.amount)"
	expectedMethod := "getData"
	expectedParams := []string{"getParams", "message.sender", "message.amount"}
	prefix, method, params, err := parser.GetPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, "smc")
	require.Equal(t, method, expectedMethod)
	require.Equal(t, params, expectedParams)
}

func TestGetPrefix_WithNestedBuiltIn(t *testing.T) {
	parser := &ksml.Parser{}
	content := "smc:getData(getParams, fn:getTimeStamp(), fn:float(message.amount), fn:int(smc:getData(getAge,John)), fn:mul(fn:int(message.amount),fromAmount))"
	expectedMethod := "getData"
	expectedParams := []string{"getParams", "fn:getTimeStamp()", "fn:float(message.amount)", "fn:int(smc:getData(getAge,John))", "fn:mul(fn:int(message.amount),fromAmount)"}
	prefix, method, params, err := parser.GetPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, "smc")
	require.Equal(t, method, expectedMethod)
	require.Equal(t, params, expectedParams)
}

func setup(sampleCode []byte, sampleDefinition string, globalPatterns []string, globalMessage *message.EventMessage) (*ksml.Parser, error) {
	dbInfo := NewMemoryDbInfo()
	db, _ := dbInfo.Start()

	genesisAccounts := make(map[string]*big.Int)
	genesisContracts := make(map[string]string)
	genesisAddress := "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"
	contractAddress := common.HexToAddress("0x0A")

	smc := &kaiType.KardiaSmartcontract{
		MasterSmc:  contractAddress.Hex(),
		SmcAddress: contractAddress.Hex(),
		MasterAbi:  sampleDefinition,
		SmcAbi:     sampleDefinition,
	}
	db.WriteEvent(smc)

	amount, _ := big.NewInt(0).SetString("1000000000000000000000000000", 10)
	genesisAccounts[genesisAddress] = amount
	genesisContracts["0x0A"] = common.Bytes2Hex(sampleCode)
	ga, err := genesis.GenesisAllocFromAccountAndContract(genesisAccounts, genesisContracts)
	if err != nil {
		return nil, err
	}

	g := &genesis.Genesis{
		Config:   configs.TestnetChainConfig,
		GasLimit: 16777216, // maximum number of uint24
		Alloc:    ga,
	}

	logger := log.New()
	stakingUtil, _ := staking.NewSmcStakingnUtil()
	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, db, g, stakingUtil)
	if genesisErr != nil {
		return nil, err
	}

	bc, err := blockchain.NewBlockChain(logger, db, chainConfig, false)
	if err != nil {
		return nil, err
	}

	txConfig := tx_pool.TxPoolConfig{
		GlobalSlots: 64,
		GlobalQueue: 5120000,
	}
	txPool := tx_pool.NewTxPool(txConfig, chainConfig, bc)

	// mock function stimulates publish function
	publishFunc := func(endpoint string, topic string, msg message2.TriggerMessage) error {
		println(fmt.Sprintf("publishing message to %v with topic %v", endpoint, topic))
		b, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		println(string(b))
		return nil
	}

	return ksml.NewParser("ETH", "0.0.0.0:5555", publishFunc, bc, txPool, &contractAddress, globalPatterns, globalMessage, true), nil
}

func TestParseParams_withReturn(t *testing.T) {
	patterns := []string{
		"${fn:var(data,bool,true)}",
		"${fn:validate(data,SIGNAL_CONTINUE,SIGNAL_RETURN)}",
		"hello",
	}
	msg := &message.EventMessage{
		Params: []string{"true"},
	}
	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedResult := []interface{}{"hello"}
	require.Equal(t, parser.GetGlobalParams(), expectedResult)
}

func TestParseParams_withContinue(t *testing.T) {

	msg := &message.EventMessage{
		Params: []string{"false"},
	}

	patterns := []string{
		"${fn:var(data,bool,message.params[0])}",
		"${fn:validate(data,SIGNAL_CONTINUE,SIGNAL_RETURN)}",
		"hello",
	}

	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedResult := make([]interface{}, 0)
	require.Equal(t, parser.GetGlobalParams(), expectedResult)
}

func TestParseParams_withStop(t *testing.T) {

	msg := &message.EventMessage{
		Params: []string{"true"},
	}

	patterns := []string{
		"${smc:getData(getSingleUintValue)}",
		"${smc:getData(getBoolValue,message.params[0])}",
		"${fn:validate(params[1],SIGNAL_CONTINUE,SIGNAL_STOP)}",
		"${smc:getData(getStringValue)}",
	}

	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.Errorf(t, err, "signal stop has been applied")
}

func TestSimulateDexReleaseEvent(t *testing.T) {
	// event message is a result generated from watcherActions
	msg := &message.EventMessage{
		Params: []string{
			"ETH",
			"NEO",
			"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
			"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y",
			"0xc123b0326e4af41026c640565c58bb2977212f40b126411525c088c89e83014f",
			"64821330000000000",
			"1571045257939",
			"100000000",
			"6482133",
			"NEO;ETH|AcLRqPTphSqSBG6aZ7evhfH9QcNdZjgJX1;0x37bbE5BA2D1C717E0df8A844c304eA4f81329e50|6482133;100000000|7eade0857bf7452516a887090b1dc8b0f14a5954bd77b3e9a9a3eb5f3121ebdf;0xc123b0326e4af41026c640565c58bb2977212f40b126411525c088c89e83014f",
		},
	}
	patterns := []string{
		"${fn:defineFunc(CalculateReleasedAmountToEth)}",
		"${fn:var(tenPoweredByTen,bigInt,fn:exp(fn:int(10),fn:int(10)))}",
		"${fn:var(tenPoweredByTwelve,bigInt,fn:exp(fn:int(10),fn:int(12)))}",
		"${fn:var(convertedAmount,bigFloat,fn:float(amounts[i]))}",
		"${fn:var(convertedAmount,bigFloat,fn:mul(fn:div(fn:float(convertedAmount),fn:float(fromAmount)),fn:float(toAmount)))}",
		"${fn:if(checkFromType,fromType == 'NEO')}",
		"${fn:var(convertedAmount,bigFloat,fn:mul(fn:float(convertedAmount),fn:float(tenPoweredByTen)))}",
		"${fn:elif(checkFromType,fromType == 'TRX')}",
		"${fn:var(convertedAmount,bigFloat,fn:mul(fn:float(convertedAmount),fn:float(tenPoweredByTwelve)))}",
		"${fn:else(checkFromType)}",
		"SIGNAL_STOP",
		"${fn:endif(checkFromType)}",
		"${fn:var(convertedAmount,bigInt,fn:int(fn:round(fn:float(convertedAmount))))}",
		"${fn:endDefineFunc(CalculateReleasedAmountToEth)}",
		// define CalculateReleasedAmountFromNeo
		"${fn:defineFunc(CalculateReleasedAmountFromNeo)}",
		"${fn:var(tenPoweredByEight,bigInt,fn:exp(fn:int(10),fn:int(8)))}",
		"${fn:var(tenPoweredBySix,bigInt,fn:exp(fn:int(10),fn:int(6)))}",
		"${fn:if(checkFromType,fromType=='ETH')}",
		"${fn:var(convertedAmount,bigInt,fn:div(fn:int(amounts[i]),fn:float(tenPoweredByEight)))}",
		"${fn:elif(checkFromType,fromType=='TRX')}",
		"${fn:var(convertedAmount,bigFloat,fn:float(amounts[i]))}",
		"${fn:var(convertedAmount,bigFloat,fn:mul(fn:float(convertedAmount),fn:float(toAmount)))}",
		"${fn:var(convertedAmount,bigFloat,fn:div(fn:float(convertedAmount),fn:float(fromAmount)))}",
		"${fn:var(convertedAmount,bigFloat,fn:div(fn:float(convertedAmount),fn:float(tenPoweredBySix)))}",
		"${fn:var(convertedAmount,int64,fn:int(fn:round(fn:int(convertedAmount))))}",
		"${fn:else(checkFromType)}",
		"SIGNAL_STOP",
		"${fn:endif(checkFromType)}",
		"${fn:validate(types[i] == 'NEO' && int(convertedAmount) < 1,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:endDefineFunc(CalculateReleasedAmountFromNeo)}",
		"${fn:var(originalTxId,string,message.params[4])}",
		"${fn:split(message.params[9],'|')}", // split released information by '|' result will be stored in global param 'params'
		"${fn:validate(size(params[0])!=4,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		//"${smc:getData(getAddressFromType,proxyName)}", // if no err return, result will be appended into params
		"${fn:var(targetContractAddress,string,'AHJoAbhenvrgSqUpfLWuwy55Lyi596MEt3')}",
		"${fn:var(fromAmount,bigInt,fn:int(message.params[7]))}",
		"${fn:var(toAmount,bigInt,fn:int(message.params[8]))}",
		"${fn:var(convertedAmount,bigInt,fn:int(0))}",
		"${fn:var(fromType,string,message.params[0])}",
		"${fn:var(toType,string,message.params[1])}",
		"${fn:var(types,list,fn:split(params[0][0],';'))}",
		"${fn:var(addresses,list,fn:split(params[0][1],';'))}",
		"${fn:var(amounts,list,fn:split(params[0][2],';'))}",
		"${fn:var(txIds,list,fn:split(params[0][3],';'))}",
		"${fn:forEach(forEachTypes,types,i)}",
		"${fn:if(checkEmpty,addresses[i]!='' && amounts[i]!='' && txIds[i]!='' && types[i]==proxyName)}",
		"${fn:if(checkFromType,fromType != proxyName)}", // swap fromAmount and toAmount
		"${fn:var(fromAmount,bigInt,fn:int(message.params[8]))}",
		"${fn:var(toAmount,bigInt,fn:int(message.params[7]))}",
		"${fn:else(checkFromType)}",
		"${fn:var(fromType,string,toType)}",
		"${fn:endif(checkFromType)}",
		"${fn:if(checkReleaseType,types[i]=='TRX')}",
		"${fn:var(convertedAmount,bigInt,fn:int(amounts[i]))}",
		"${fn:elif(checkReleaseType,types[i]=='ETH')}",
		"${fn:call(CalculateReleasedAmountToEth)}",
		"${fn:elif(checkReleaseType,types[i]=='NEO')}",
		"${fn:call(CalculateReleasedAmountFromNeo)}",
		"${fn:else(checkReleaseType)}",
		"SIGNAL_STOP",
		"${fn:endif(checkReleaseType)}",
		"${fn:var(convertedAmount,string,convertedAmount)}",
		"${[targetContractAddress,'release',[addresses[i],convertedAmount]]}", // return a list to params[0]
		"${fn:var(triggerMessage,list,params[0])}",
		"${['contractAddress','updateTargetTx',[originalTxId]]}", // return a list of callback to params[1]
		"${fn:var(cbs,list,[params[1]])}",
		"${fn:publish(triggerMessage[0],triggerMessage[1],triggerMessage[2],cbs)}",
		"${fn:endif(checkEmpty)}",
		"${fn:endForEach(forEachTypes)}",
	}
	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
}

func TestSimulateDexDepositETHEvent(t *testing.T) {
	// assume that all contract call/trigger are correct. Only test normal actions
	// event message is a result generated from watcherActions
	msg := &message.EventMessage{
		From:          "ETH",
		To:            "NEO",
		Amount:        64821330000000000,
		TransactionId: "0xc123b0326e4af41026c640565c58bb2977212f40b126411525c088c89e83014f",
		Sender:        "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
		Params: []string{
			"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y",
			"NEO",
		},
	}

	patterns := []string{
		"${fn:var(tenPoweredByTen,bigInt,fn:exp(fn:int(10),fn:int(10)))}",
		"${fn:var(tenPoweredByTwelve,bigInt,fn:exp(fn:int(10),fn:int(12)))}",
		"${fn:var(timeStamp,int64,fn:currentTimeStamp())}",
		"100000000",
		"6482133",
		//"${smc:getData(getRate,message.from,message.to)}",
		"${fn:var(fromAmount,bigInt,params[0])}",
		"${fn:var(toAmount,bigInt,params[1])}",
		"${fn:var(zeroValue,bigInt,0)}",
		"${fn:cmp(fromAmount,zeroValue,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:cmp(toAmount,zeroValue,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:var(convertedAmount,bigInt,fn:mul(fn:int(message.amount),fn:int(fromAmount)))}",
		"${fn:var(convertedAmount,bigInt,fn:div(fn:int(convertedAmount),fn:int(toAmount)))}",
		"${fn:if(evaluateDestination,message.to=='NEO')}",
		"${fn:var(convertedAmount,bigInt,fn:div(fn:int(convertedAmount),fn:int(tenPoweredByTen)))}",
		"${fn:elif(evaluateDestination,message.to=='TRX')}",
		"${fn:var(convertedAmount,bigInt,fn:div(fn:int(convertedAmount),fn:int(tenPoweredByTwelve)))}",
		"${fn:else(evaluateDestination)}",
		"SIGNAL_STOP",
		"${fn:endif(evaluateDestination)}",
		"${['addOrder', message.from, message.to, message.sender, message.params[0],message.transactionId,convertedAmount,timeStamp]}",
		//"${fn:var(addOrderTx,string,smc:trigger(addOrder,message.from,message.to,message.params[0],message.receiver,message.transactionId,convertedAmount,timeStamp))}",
		//"${smc:trigger(updateKardiaTx,message.transactionId,addOrderTx)}",
	}
	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
}

func TestSimulateDexDepositTRX(t *testing.T) {
	// assume that all contract call/trigger are correct. Only test normal actions
	// event message is a result generated from watcherActions
	msg := &message.EventMessage{
		From:          "TRX",
		To:            "ETH",
		Amount:        16571000,
		TransactionId: "375b6ee6ce8ccfd28ec7805b91190245ab926a04397bbaef6643627d658adf9f",
		Sender:        "TMm48VEk8foxwo5656DsAS58WKtgV7fYtH",
		Params: []string{
			"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
			"ETH",
		},
	}

	patterns := []string{
		"${fn:var(timeStamp,bigInt,fn:currentTimeStamp())}",
		//"${smc:getData(message.from,message.to)}",
		"100000000",
		"16571",
		"${fn:var(fromAmount,bigInt,params[0])}",
		"${fn:var(toAmount,bigInt,params[1])}",
		"${fn:var(zeroValue,bigInt,0)}",
		"${fn:cmp(fromAmount,zeroValue,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:cmp(toAmount,zeroValue,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:var(convertedAmount,bigInt,fn:int(message.amount))}",
		//"${fn:var(addOrderTx,string,smc:trigger(addOrder,message.from,message.to,message.sender,message.params[0],message.transactionId,convertedAmount,timeStamp))}",
		//"${smc:trigger(updateKardiaTx,message.transactionId,addOrderTx)}",
	}

	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
}

func TestSimulateDexDepositNEO(t *testing.T) {
	// assume that all contract call/trigger are correct. Only test normal actions
	// event message is a result generated from watcherActions
	msg := &message.EventMessage{
		From:          "NEO",
		To:            "ETH",
		Amount:        6482133,
		TransactionId: "728f62b40d778ac1dcaadf912a91fbbd6b9d4550723ec91b5089c950524fc087",
		Sender:        "AHJoAbhenvrgSqUpfLWuwy55Lyi596MEt3",
		Params: []string{
			"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
			"ETH",
		},
	}

	patterns := []string{
		"${fn:var(tenPoweredByEight,bigInt,fn:exp(fn:int(10),fn:int(8)))}",
		"${fn:var(tenPoweredBySix,bigInt,fn:exp(fn:int(10),fn:int(6)))}",
		"${fn:var(timeStamp,bigInt,fn:currentTimeStamp())}",
		"${fn:var(convertedAmount,bigInt,0)}",
		"100000000",
		"6482133",
		"${fn:var(fromAmount,bigInt,params[0])}",
		"${fn:var(toAmount,bigInt,params[1])}",
		"${fn:var(zeroValue,bigInt,0)}",
		"${fn:cmp(fromAmount,zeroValue,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:cmp(toAmount,zeroValue,SIGNAL_STOP,SIGNAL_CONTINUE)}",
		"${fn:if(evaluateDestination,message.to=='ETH')}",
		"${fn:var(convertedAmount,bigInt,fn:mul(fn:int(message.amount),fn:int(tenPoweredByEight)))}",
		"${fn:elif(evaluateDestination,message.to=='TRX')}",
		"${fn:var(rateFloat,float64,fn:format(fn:div(fn:float(fromAmount),fn:float(toAmount)),6))}",
		"${fn:var(rateInt,bigInt,fn:int(fn:mul(rateFloat,fn:float(tenPoweredBySix)))}",
		"${fn:var(convertedAmount,bigInt,fn:mul(rateInt,fn:int(message.amount)))}",
		"${fn:else(evaluateDestination)}",
		"SIGNAL_STOP",
		"${fn:endif(evaluateDestination)}",
		//"${fn:var(addOrderTx,string,smc:trigger(addOrder,message.from,message.to,message.sender,message.params[0],message.transactionId,convertedAmount,timeStamp))}",
		//"${smc:trigger(updateKardiaTx,message.transactionId,addOrderTx)}",
	}

	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)
}
