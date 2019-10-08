package ksml

import (
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/ksml/proto"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/types"
	kaiType "github.com/kardiachain/go-kardia/types"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

type MemoryDbInfo struct {}


func NewMemoryDbInfo() *MemoryDbInfo {
	return &MemoryDbInfo{}
}

func (db *MemoryDbInfo) Name() string {
	return "Memory"
}

func (db *MemoryDbInfo) Start() (types.Database, error) {
	return types.NewMemStore(), nil
}

func TestGetPrefix_WithoutPrefix(t *testing.T) {
	parser := Parser{}
	content := "1"
	prefix, method, params, err := parser.getPrefix(content)
	require.NoError(t, err)
	require.Len(t, prefix, 0)
	require.Equal(t, method, content)
	require.Nil(t, params)
}

func TestGetPrefix_WithValidPrefix(t *testing.T) {
	parser := Parser{}
	content := "fn:currentTimeStamp()"
	prefix, method, params, err := parser.getPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, builtInFn)
	require.Equal(t, method, "currentTimeStamp")
	require.Nil(t, params)
}

func TestGetPrefix_WithParam(t *testing.T) {
	parser := Parser{}
	content := "smc:getParams(message.sender)"
	expectedMethod := "getParams"
	expectedParams := []string{"message.sender"}
	prefix, method, params, err := parser.getPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, builtInSmc)
	require.Equal(t, method, expectedMethod)
	require.Equal(t, params, expectedParams)
}

func TestGetPrefix_WithParams(t *testing.T) {
	parser := &Parser{}
	content := "smc:getParams(message.sender, message.amount)"
	expectedMethod := "getParams"
	expectedParams := []string{"message.sender", "message.amount"}
	prefix, method, params, err := parser.getPrefix(content)
	require.NoError(t, err)
	require.Equal(t, prefix, builtInSmc)
	require.Equal(t, method, expectedMethod)
	require.Equal(t, params, expectedParams)
}

func setup(sampleCode []byte, sampleDefinition string, globalPatterns []string, globalMessage *message.EventMessage) (*Parser, error) {
	dbInfo := NewMemoryDbInfo()
	db, _ := dbInfo.Start()

	genesisAccounts := make(map[string]*big.Int)
	genesisContracts := make(map[string]string)
	genesisAddress := "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"
	privKey, _ := crypto.HexToECDSA("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	contractAddress := common.HexToAddress("0x0A")

	smc := &kaiType.KardiaSmartcontract{
		SmcAddress:     contractAddress.Hex(),
		SmcAbi:         sampleDefinition,
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

	baseAccount := &types.BaseAccount{
		Address:    common.HexToAddress(genesisAddress),
		PrivateKey: *privKey,
	}

	logger := log.New()

	chainConfig, _, genesisErr := genesis.SetupGenesisBlock(logger, db, g, baseAccount)
	if genesisErr != nil {
		return nil, err
	}

	bc, err := blockchain.NewBlockChain(logger, db, chainConfig, false)
	if err != nil {
		return nil, err
	}

	statedb, err := bc.StateAt(bc.CurrentBlock().Root())
	if err != nil {
		return nil, err
	}
	return NewParser(bc, statedb, &contractAddress, globalPatterns,globalMessage), nil
}

func TestParseParams_withReturn(t *testing.T) {
	patterns := []string{
		"${smc:getSingleUintValue()}",
		"${smc:getBoolValue(message.params[0])}",
		"${fn:validate(params[1],SIGNAL_CONTINUE,SIGNAL_RETURN)}",
		"${smc:getStringValue()}",
	}
	msg := &message.EventMessage{
		Params: []string{"true"},
	}
	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedResult := []interface{}{uint8(1), false}
	require.Equal(t, parser.globalParams, expectedResult)
}

func TestParseParams_withContinue(t *testing.T) {

	msg := &message.EventMessage{
		Params: []string{"false"},
	}

	patterns := []string{
		"${smc:getSingleUintValue()}",
		"${smc:getBoolValue(message.params[0])}",
		"${fn:validate(params[1],SIGNAL_CONTINUE,SIGNAL_RETURN)}",
		"${smc:getStringValue()}",
	}

	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.NoError(t, err)

	expectedResult := []interface{}{uint8(1), true, "hello"}
	require.Equal(t, parser.globalParams, expectedResult)
}

func TestParseParams_withStop(t *testing.T) {

	msg := &message.EventMessage{
		Params: []string{"true"},
	}

	patterns := []string{
		"${smc:getSingleUintValue()}",
		"${smc:getBoolValue(message.params[0])}",
		"${fn:validate(params[1],SIGNAL_CONTINUE,SIGNAL_STOP)}",
		"${smc:getStringValue()}",
	}

	parser, err := setup(sampleCode4, sampleDefinition4, patterns, msg)
	require.NoError(t, err)

	err = parser.ParseParams()
	require.Errorf(t, err, "signal stop has been applied")
}
