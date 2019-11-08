package contracts

import (
	"fmt"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"
)

func testDeployGenesisNodesAndStakes(t *testing.T, bc *blockchain.BlockChain, st *state.StateDB) {
	kAbi, err := abi.JSON(strings.NewReader(NodeAbi))
	require.NoError(t, err)

	for _, node := range genesisNodes {
		addressHex := node["address"].(string)
		owner := node["owner"].(string)
		id := node["id"].(string)
		name := node["name"].(string)
		host := node["host"].(string)
		port := node["port"].(string)
		percentageReward := node["percentageReward"].(uint16)
		staker := node["staker"].(string)

		input, err := kAbi.Pack("", masterAddress, common.HexToAddress(owner), id, name, host, port, percentageReward)
		require.NoError(t, err)

		newCode := append(NodeByteCode, input...)
		address := common.HexToAddress(addressHex)
		// Setup contract code into genesis state
		_, _, _, err = create(common.HexToAddress(owner), address, bc.CurrentHeader(), bc, newCode, st)
		require.NoError(t, err)

		stakerAbi, err := abi.JSON(strings.NewReader(StakerAbi))
		require.NoError(t, err)

		input, err = stakerAbi.Pack("", masterAddress, common.HexToAddress(owner), big.NewInt(100), minimumStakes)
		require.NoError(t, err)

		newStakerCode := append(StakerByteCode, input...)
		stakerAddress := common.HexToAddress(staker)
		_, _, _, err = create(common.HexToAddress(owner), stakerAddress, bc.CurrentHeader(), bc, newStakerCode, st)
		require.NoError(t, err)

		stakeInput, err := stakerAbi.Pack("stake", address)
		require.NoError(t, err)

		_, err = call(common.HexToAddress(owner), stakerAddress, bc.CurrentHeader(), bc, stakeInput, minimumStakes, st)
		require.NoError(t, err)

		getStakeAmount, err := stakerAbi.Pack("getStakeAmount", address)
		require.NoError(t, err)

		result, err := staticCall(common.HexToAddress(owner), stakerAddress, bc.CurrentHeader(), bc, getStakeAmount, st)
		require.NoError(t, err)

		type data struct {
			Amount *big.Int
			Valid  bool
		}
		var actualData data
		err = stakerAbi.Unpack(&actualData, "getStakeAmount", result)
		require.NoError(t, err)

		expectedData := data {
			Amount: minimumStakes,
			Valid: true,
		}

		require.Equal(t, expectedData.Amount.String(), actualData.Amount.String())
		require.Equal(t, expectedData.Valid, actualData.Valid)
	}
}

func testAddStaker(t *testing.T, kAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB) {
	// add staker and stake
	for i, node := range genesisNodes {
		staker := common.HexToAddress(node["staker"].(string))
		addStaker, err := kAbi.Pack("addStaker", staker)
		require.NoError(t, err)

		_, err = call(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, addStaker, big.NewInt(0), st)
		require.NoError(t, err)

		isStaker, err := kAbi.Pack("IsStaker", staker)
		require.NoError(t, err)

		isStakerResult, err := staticCall(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, isStaker, st)
		require.NoError(t, err)

		var isBool bool
		err = kAbi.Unpack(&isBool, "IsStaker", isStakerResult)
		require.Equal(t, true, isBool)

		IsAvailableNodes, err := kAbi.Pack("IsAvailableNodes", common.HexToAddress(node["address"].(string)))
		require.NoError(t, err)

		IsAvailableNodesResult, err := staticCall(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, IsAvailableNodes, st)
		require.NoError(t, err)

		var index uint64
		err = kAbi.Unpack(&index, "IsAvailableNodes", IsAvailableNodesResult)
		require.NoError(t, err)
		require.Equal(t, uint64(i+1), index)

		getTotalStakes, err := kAbi.Pack("getTotalStakes", common.HexToAddress(node["address"].(string)))
		require.NoError(t, err)

		result, err := staticCall(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, getTotalStakes, st)
		require.NoError(t, err)

		var actual *big.Int
		err = kAbi.Unpack(&actual, "getTotalStakes", result)
		require.NoError(t, err)

		expected := big.NewInt(0)
		require.Equal(t, expected.String(), actual.String())
	}
}

func testCreateMaster(t *testing.T, kAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB) {
	input, err := kAbi.Pack("", uint64(10), uint64(3))
	require.NoError(t, err)
	sender := common.HexToAddress(genesisNodes[0]["owner"].(string))
	newCode := append(MasterByteCode, input...)
	_, _, _, err = create(sender, masterAddress, bc.CurrentHeader(), bc, newCode, st)
	require.NoError(t, err)

	// check _availableNodes
	getTotalAvailableNodes, err := kAbi.Pack("GetTotalAvailableNodes")
	require.NoError(t, err)

	result, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, getTotalAvailableNodes, st)
	require.NoError(t, err)

	var totalAvailableNodes uint64
	err = kAbi.Unpack(&totalAvailableNodes, "GetTotalAvailableNodes", result)
	require.NoError(t, err)
	require.Equal(t, uint64(3), totalAvailableNodes)
}

func testGetTotalStakes(t *testing.T, kAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, expected *big.Int) {
	for _, node := range genesisNodes {
		getTotalStakes, err := kAbi.Pack("getTotalStakes", common.HexToAddress(node["address"].(string)))
		require.NoError(t, err)

		result, err := staticCall(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, getTotalStakes, st)
		require.NoError(t, err)

		var actual *big.Int
		err = kAbi.Unpack(&actual, "getTotalStakes", result)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}

func testCollectValidators(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB) {
	input, err := masterAbi.Pack("collectValidators")
	require.NoError(t, err)

	sender := common.HexToAddress(genesisNodes[0]["owner"].(string))
	senderNode := common.HexToAddress(genesisNodes[0]["address"].(string))

	_, err = call(sender, masterAddress, bc.CurrentHeader(), bc, input, big.NewInt(0), st)
	require.NoError(t, err)

	isValidator, err := masterAbi.Pack("IsValidator", sender)
	require.NoError(t, err)

	result, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, isValidator, st)
	require.NoError(t, err)

	var actual bool
	err = masterAbi.Unpack(&actual, "IsValidator", result)
	require.Equal(t, true, actual)

	isValidator, err = masterAbi.Pack("IsValidator", senderNode)
	require.NoError(t, err)

	result, err = staticCall(sender, masterAddress, bc.CurrentHeader(), bc, isValidator, st)
	require.NoError(t, err)

	err = masterAbi.Unpack(&actual, "IsValidator", result)
	require.Equal(t, true, actual)
}

func testGetLatestValidators(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB) {
	println("running testGetLatestValidators")
	sender := common.HexToAddress(genesisNodes[0]["owner"].(string))

	getLatestValidatorsLength, err := masterAbi.Pack("getLatestValidatorsLength")
	require.NoError(t, err)

	result, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, getLatestValidatorsLength, st)
	require.NoError(t, err)

	var validatorsLength uint64
	err = masterAbi.Unpack(&validatorsLength, "getLatestValidatorsLength", result)
	require.NoError(t, err)
	require.Equal(t, uint64(4), validatorsLength)

	for i:=uint64(1); i < validatorsLength; i++ {
		getLatestValidator, err := masterAbi.Pack("GetLatestValidator", i)
		require.NoError(t, err)

		result, err = staticCall(sender, masterAddress, bc.CurrentHeader(), bc, getLatestValidator, st)
		require.NoError(t, err)
		type validator struct {
			Node common.Address `abi:"node"`
			Owner common.Address `abi:"owner"`
			Stakes *big.Int `abi:"stakes"`
			TotalStaker uint64 `abi:"totalStaker"`
		}
		var actual validator
		err = masterAbi.Unpack(&actual, "GetLatestValidator", result)
		require.NoError(t, err)

		node := genesisNodes[i-1]
		println(fmt.Sprintf("node:%v owner:%v stakes:%v totalStaker:%v", actual.Node.Hex(), actual.Owner.Hex(), actual.Stakes, actual.TotalStaker))
		require.Equal(t, node["address"].(string), actual.Node.Hex())
		require.Equal(t, node["owner"].(string), actual.Owner.Hex())
		require.Equal(t, minimumStakes, actual.Stakes)
		require.Equal(t, uint64(2), actual.TotalStaker)
	}
}

func setup(t *testing.T) (*blockchain.BlockChain, abi.ABI, *state.StateDB) {
	bc, err := setupBlockchain()
	require.NoError(t, err)

	// setup Master smc
	masterAbi, err := abi.JSON(strings.NewReader(MasterAbi))
	require.NoError(t, err)

	st, err := bc.State()
	require.NoError(t, err)

	return bc, masterAbi, st
}

func TestMaster(t *testing.T) {
	bc, masterAbi, st := setup(t)
	testCreateMaster(t, masterAbi, bc, st)
	testAddStaker(t, masterAbi, bc, st)
	testDeployGenesisNodesAndStakes(t, bc, st)
	testGetTotalStakes(t, masterAbi, bc, st, minimumStakes)
	testCollectValidators(t, masterAbi, bc, st)
	testGetLatestValidators(t, masterAbi, bc, st)
}
