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

func testDeployStaker(t *testing.T, bc *blockchain.BlockChain, st *state.StateDB, node map[string]interface{}) {
	stakerAbi, err := abi.JSON(strings.NewReader(StakerAbi))
	require.NoError(t, err)
	owner := common.HexToAddress(node["owner"].(string))
	staker := common.HexToAddress(node["staker"].(string))
	input, err := stakerAbi.Pack("", masterAddress, owner, big.NewInt(100), minimumStakes)
	require.NoError(t, err)
	newStakerCode := append(StakerByteCode, input...)
	_, _, _, err = create(owner, staker, bc.CurrentHeader(), bc, newStakerCode, big.NewInt(0), st)
	require.NoError(t, err)
}

func testStake(t *testing.T, bc *blockchain.BlockChain, st *state.StateDB, node map[string]interface{}, target *common.Address, stakeAmount, expectedStakes *big.Int) {
	address := common.HexToAddress(node["address"].(string))
	if target != nil {
		address = *target
	}
	staker := common.HexToAddress(node["staker"].(string))
	owner := common.HexToAddress(node["owner"].(string))

	println(fmt.Sprintf("testStake address:%v staker:%v owner:%v", address.Hex(), staker.Hex(), owner.Hex()))

	stakerAbi, err := abi.JSON(strings.NewReader(StakerAbi))
	require.NoError(t, err)

	stakeInput, err := stakerAbi.Pack("stake", address)
	require.NoError(t, err)

	_, err = call(owner, staker, bc.CurrentHeader(), bc, stakeInput, stakeAmount, st)
	require.NoError(t, err)

	getStakeAmount, err := stakerAbi.Pack("getStakeAmount", address)
	require.NoError(t, err)

	result, err := staticCall(owner, staker, bc.CurrentHeader(), bc, getStakeAmount, st)
	require.NoError(t, err)

	type data struct {
		Amount *big.Int
		Valid  bool
	}
	var actualData data
	err = stakerAbi.Unpack(&actualData, "getStakeAmount", result)
	require.NoError(t, err)

	expectedData := data {
		Amount: expectedStakes,
		Valid: true,
	}

	require.Equal(t, expectedData.Amount.String(), actualData.Amount.String())
	require.Equal(t, expectedData.Valid, actualData.Valid)
}

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

		input, err := kAbi.Pack("", masterAddress, common.HexToAddress(owner), id, name, host, port, percentageReward)
		require.NoError(t, err)

		newCode := append(NodeByteCode, input...)
		address := common.HexToAddress(addressHex)
		// Setup contract code into genesis state
		_, _, _, err = create(common.HexToAddress(owner), address, bc.CurrentHeader(), bc, newCode, big.NewInt(0), st)
		require.NoError(t, err)

		testDeployStaker(t, bc, st, node)
		testStake(t, bc, st, node, nil, minimumStakes, minimumStakes)
	}
}

func testAddGenesisStaker(t *testing.T, kAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB) {
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

func testAddStaker(t *testing.T, kAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB) {
	genesisSender := common.HexToAddress(genesisNodes[0]["owner"].(string))
	for _, node := range normalNodes {
		staker := common.HexToAddress(node["staker"].(string))
		addStaker, err := kAbi.Pack("addStaker", staker)
		require.NoError(t, err)

		_, err = call(genesisSender, masterAddress, bc.CurrentHeader(), bc, addStaker, big.NewInt(0), st)
		require.NoError(t, err)

		isStaker, err := kAbi.Pack("IsStaker", staker)
		require.NoError(t, err)

		isStakerResult, err := staticCall(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, isStaker, st)
		require.NoError(t, err)

		var isBool bool
		err = kAbi.Unpack(&isBool, "IsStaker", isStakerResult)
		require.Equal(t, true, isBool)

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

func testAvailableNodes(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, expectedLen uint64) {
	sender := common.HexToAddress(genesisNodes[0]["owner"].(string))
	getTotalAvailableNodes, err := masterAbi.Pack("GetTotalAvailableNodes")
	require.NoError(t, err)

	result, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, getTotalAvailableNodes, st)
	require.NoError(t, err)

	var totalAvailableNodes *big.Int
	err = masterAbi.Unpack(&totalAvailableNodes, "GetTotalAvailableNodes", result)
	require.NoError(t, err)
	require.Equal(t, expectedLen, totalAvailableNodes.Uint64())

	for i:=uint64(1); i<=totalAvailableNodes.Uint64(); i++ {
		input, err := masterAbi.Pack("getAvailableNode", big.NewInt(0).SetUint64(i))
		require.NoError(t, err)
		output, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
		require.NoError(t, err)
		type nodeInfo struct {
			NodeAddress common.Address `abi:"nodeAddress"`
			Owner common.Address `abi:"owner"`
			Stakes *big.Int `abi:"stakes"`
		}
		var info nodeInfo
		err = masterAbi.Unpack(&info, "getAvailableNode", output)
		require.NoError(t, err)
		println(fmt.Sprintf("available node by index - index:%v node:%v owner:%v stakes:%v", i, info.NodeAddress.Hex(), info.Owner.Hex(), info.Stakes.String()))
		testGetAvailableNodeIndex(t, masterAbi, bc, st, info.NodeAddress, i)
	}
}

func testGetAvailableNodeIndex(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, node common.Address, expectedIndex uint64) {
	input, err := masterAbi.Pack("getAvailableNodeIndex", node)
	require.NoError(t, err)
	output, err := staticCall(node, masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)
	var index *big.Int
	err = masterAbi.Unpack(&index, "getAvailableNodeIndex", output)
	require.NoError(t, err)
	require.Equal(t, expectedIndex, index.Uint64())
}

func testCreateMaster(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, consensusPeriod uint64, maxValidators uint64) {
	input, err := masterAbi.Pack("", consensusPeriod, maxValidators)
	require.NoError(t, err)
	//sender := common.HexToAddress(genesisNodes[0]["owner"].(string))
	sender := common.HexToAddress("0x")
	newCode := append(MasterByteCode, input...)
	_, _, _, err = create(sender, masterAddress, bc.CurrentHeader(), bc, newCode, genesisAmount, st)
	require.NoError(t, err)

	// check _availableNodes
	testAvailableNodes(t, masterAbi, bc, st, uint64(len(genesisNodes)))
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

func testGetLatestValidators(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, expectedValidatorsLength uint64, expectedNodes []map[string]interface{}) {
	println("running testGetLatestValidators")
	sender := common.HexToAddress(genesisNodes[0]["owner"].(string))

	getLatestValidatorsInfo, err := masterAbi.Pack("getLatestValidatorsInfo")
	require.NoError(t, err)

	result, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, getLatestValidatorsInfo, st)
	require.NoError(t, err)
	type getLatestValidatorsInfoType struct {
		TotalNodes uint64 `abi:"totalNodes"`
		StartAtBlock uint64 `abi:"startAtBlock"`
		EndAtBlock uint64 `abi:"endAtBlock"`
	}
	var validatorsInfo getLatestValidatorsInfoType
	err = masterAbi.Unpack(&validatorsInfo, "getLatestValidatorsInfo", result)
	require.NoError(t, err)
	require.Equal(t, expectedValidatorsLength, validatorsInfo.TotalNodes)

	for i:=uint64(1); i < validatorsInfo.TotalNodes; i++ {
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

		node := expectedNodes[i-1]
		println(fmt.Sprintf("node:%v owner:%v stakes:%v totalStaker:%v", actual.Node.Hex(), actual.Owner.Hex(), actual.Stakes, actual.TotalStaker))
		require.Equal(t, node["address"].(string), actual.Node.Hex())
		require.Equal(t, node["owner"].(string), actual.Owner.Hex())
		require.Equal(t, node["expectedStakes"].(*big.Int), actual.Stakes)
		require.Equal(t, node["expectedStaker"].(uint64), actual.TotalStaker)
	}
}

func testGetPendingNode(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB,
	index uint64, expectedAddress common.Address, expectedVote uint64) {
	input, err := masterAbi.Pack("GetPendingNode", index)
	require.NoError(t, err)

	output, err := staticCall(common.HexToAddress(genesisNodes[0]["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)

	type pendingNode struct {
		NodeAddress common.Address `abi:"nodeAddress"`
		Stakes *big.Int `abi:"stakes"`
		Vote uint64 `abi:"vote"`
	}
	var outputNode pendingNode
	err = masterAbi.Unpack(&outputNode, "GetPendingNode", output)
	require.NoError(t, err)

	if outputNode.NodeAddress.Equal(common.HexToAddress("0x")) {
		return
	}

	expectedNode := pendingNode{
		NodeAddress: expectedAddress,
		Stakes:      big.NewInt(0),
		Vote:        expectedVote,
	}

	require.NotNil(t, outputNode)
	println(fmt.Sprintf("finish getting pending node address:%v vote:%v", outputNode.NodeAddress.Hex(), outputNode.Vote))

	require.Equal(t, expectedNode.NodeAddress.Hex(), outputNode.NodeAddress.Hex())
	require.Equal(t, expectedNode.Stakes.String(), outputNode.Stakes.String())
	require.Equal(t, expectedNode.Vote, outputNode.Vote)
}

func testAddPendingNode(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, node map[string]interface{}, sender common.Address) {
	address := common.HexToAddress(node["address"].(string))
	owner := common.HexToAddress(node["owner"].(string))

	input, err := masterAbi.Pack("addPendingNode", address, owner)
	require.NoError(t, err)

	_, err = call(sender, masterAddress, bc.CurrentHeader(), bc, input, big.NewInt(0), st)
	require.NoError(t, err)

	input, err = masterAbi.Pack("GetTotalPending")
	require.NoError(t, err)

	output, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)

	var result *big.Int
	err = masterAbi.Unpack(&result, "GetTotalPending", output)
	require.NoError(t, err)
	require.Equal(t, true, result.Uint64() > 0)

	println(fmt.Sprintf("finish testAddPendingNode sender:%v address:%v owner:%v", sender.Hex(), address.Hex(), owner.Hex()))
	//testGetPendingNode(t, masterAbi, bc, st, result, address, uint64(1))
}

func testVotePending(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, nodes []map[string]interface{},
		expectedAvailableLen uint64) {
	sender := common.HexToAddress(genesisNodes[0]["owner"].(string))
	// get latest pending node
	input, err := masterAbi.Pack("GetTotalPending")
	require.NoError(t, err)

	output, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)

	var index *big.Int
	err = masterAbi.Unpack(&index, "GetTotalPending", output)
	require.NoError(t, err)
	require.Equal(t, true, index.Uint64() > 0)

	for _, node := range nodes {
		input, err = masterAbi.Pack("votePending", index.Uint64())
		require.NoError(t, err)
		println(fmt.Sprintf("voting for index:%v sender:%v", index.Uint64(), node["owner"].(string)))
		_, err = call(common.HexToAddress(node["owner"].(string)), masterAddress, bc.CurrentHeader(), bc, input, big.NewInt(0), st)
		if err != nil {
			// try to get pending node by index, if it is found then throw t.Fatal
			input, err = masterAbi.Pack("GetPendingNode", index.Uint64())
			require.NoError(t, err)
			output, err = staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
			require.NoError(t, err)
			type pendingNode struct {
				NodeAddress common.Address `abi:"nodeAddress"`
				Stakes *big.Int `abi:"stakes"`
				Vote uint64 `abi:"vote"`
			}
			var outputNode pendingNode
			err = masterAbi.Unpack(&outputNode, "GetPendingNode", output)
			require.NoError(t, err)
			if !outputNode.NodeAddress.Equal(common.HexToAddress("0x")) {
				t.Fatal("expected pending node does not exist, but got existed")
			}
		}
	}
	if expectedAvailableLen > 0 {
		// check available nodes
		testAvailableNodes(t, masterAbi, bc, st, expectedAvailableLen)
	}
}

func testHasPendingVoted(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, sender common.Address, index uint64, expected bool) {
	input, err := masterAbi.Pack("hasPendingVoted", index)
	require.NoError(t, err)

	output, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)

	var result bool
	err = masterAbi.Unpack(&result, "hasPendingVoted", output)
	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func testRequestDelete(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, index uint64, sender, expectedNode common.Address) {
	input, err := masterAbi.Pack("requestDelete", index)
	require.NoError(t, err)

	_, err = call(sender, masterAddress, bc.CurrentHeader(), bc, input, big.NewInt(0), st)
	require.NoError(t, err)

	testGetRequestDelete(t, masterAbi, bc, st, uint64(1), index, uint64(1), sender, expectedNode)
}

func testGetRequestDelete(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, index, expectedIndex, expectedVote uint64, sender, expectedNode common.Address) {
	input, err := masterAbi.Pack("getRequestDeleteNode", index)
	require.NoError(t, err)
	output, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)

	type deleteInfo struct {
		NodeIndex uint64 `abi:"nodeIndex"`
		NodeAddress common.Address `abi:"nodeAddress"`
		Stakes *big.Int `abi:"stakes"`
		Vote uint64 `abi:"vote"`
	}
	var info deleteInfo
	err = masterAbi.Unpack(&info, "getRequestDeleteNode", output)
	require.NoError(t, err)

	println(fmt.Sprintf("testGetRequestDelete index:%v nodeIndex:%v nodeAddress:%v stakes:%v vote:%v", index, info.NodeIndex, info.NodeAddress.Hex(), info.Stakes.Uint64(), info.Vote))
	require.Equal(t, expectedNode.Hex(), info.NodeAddress.Hex())
	require.Equal(t, expectedIndex, info.NodeIndex)
	require.Equal(t, expectedVote, info.Vote)
}

func testVoteDeleteNode(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, index, expectedIndex, expectedVote uint64, sender, expectedNode common.Address) {
	method := "voteDeleting"
	input, err := masterAbi.Pack(method, index)
	require.NoError(t, err)
	_, err = call(sender, masterAddress, bc.CurrentHeader(), bc, input, big.NewInt(0), st)
	require.NoError(t, err)

	testGetRequestDelete(t, masterAbi, bc, st, index, expectedIndex, expectedVote, sender, expectedNode)
}

func testWithdraw(t *testing.T, masterAbi abi.ABI, bc *blockchain.BlockChain, st *state.StateDB, node, staker common.Address, amount *big.Int, expectedNewIndex uint64) {
	println("start testWithdraw")
	input, err := masterAbi.Pack("withdraw", node, amount)
	require.NoError(t, err)

	_, err = call(staker, masterAddress, bc.CurrentHeader(), bc, input, big.NewInt(0), st)
	require.NoError(t, err)

	input, err = masterAbi.Pack("getAvailableNode", big.NewInt(0).SetUint64(expectedNewIndex))
	require.NoError(t, err)
	output, err := staticCall(staker, masterAddress, bc.CurrentHeader(), bc, input, st)
	require.NoError(t, err)
	type nodeInfo struct {
		NodeAddress common.Address `abi:"nodeAddress"`
		Owner common.Address `abi:"owner"`
		Stakes *big.Int `abi:"stakes"`
	}
	var info nodeInfo
	err = masterAbi.Unpack(&info, "getAvailableNode", output)
	require.NoError(t, err)
	println(fmt.Sprintf("testWithdraw - available node - index:%v node:%v owner:%v stakes:%v", expectedNewIndex, info.NodeAddress.Hex(), info.Owner.Hex(), info.Stakes.Uint64()))
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
	testCreateMaster(t, masterAbi, bc, st, uint64(10), uint64(4))
	testAddGenesisStaker(t, masterAbi, bc, st)
	testDeployGenesisNodesAndStakes(t, bc, st)
	testGetTotalStakes(t, masterAbi, bc, st, minimumStakes)
	testCollectValidators(t, masterAbi, bc, st)
	testGetLatestValidators(t, masterAbi, bc, st, uint64(3), genesisNodes)
	testAddStaker(t, masterAbi, bc, st)
	testAddPendingNode(t, masterAbi, bc, st, normalNodes[0], common.HexToAddress(genesisNodes[0]["owner"].(string)))
	testGetPendingNode(t, masterAbi, bc, st, 1, common.HexToAddress(normalNodes[0]["address"].(string)), uint64(1))
	testVotePending(t, masterAbi, bc, st, []map[string]interface{}{genesisNodes[1]}, uint64(len(genesisNodes)))
	testVotePending(t, masterAbi, bc, st, []map[string]interface{}{genesisNodes[2]}, uint64(len(genesisNodes) + 1))
	testGetPendingNode(t, masterAbi, bc, st, 1, common.HexToAddress(normalNodes[0]["address"].(string)), uint64(3))
	testDeployStaker(t, bc, st, normalNodes[0])
	testStake(t, bc, st, normalNodes[0], nil, minimumStakes, minimumStakes)
	testCollectValidators(t, masterAbi, bc, st)
	testGetLatestValidators(t, masterAbi, bc, st, uint64(4), append(genesisNodes, normalNodes[0]))

	// stakes to genesis[0] from recently added node.
	target := common.HexToAddress(genesisNodes[1]["address"].(string))
	testStake(t, bc, st, normalNodes[0], &target, minimumStakes, minimumStakes)
	testCollectValidators(t, masterAbi, bc, st)
	expectedNode := genesisNodes[1]
	expectedNode["expectedStakes"] = big.NewInt(0).Add(minimumStakes, minimumStakes)
	expectedNode["expectedStaker"] = uint64(3)
	testGetLatestValidators(t, masterAbi, bc, st, uint64(4), []map[string]interface{}{expectedNode, genesisNodes[0], genesisNodes[2], normalNodes[0]})

	// add the last node to pending
	println("add the last node to pending")
	testAddPendingNode(t, masterAbi, bc, st, normalNodes[1], common.HexToAddress(normalNodes[0]["owner"].(string)))
	testHasPendingVoted(t, masterAbi, bc, st, common.HexToAddress(genesisNodes[0]["owner"].(string)), uint64(2), false)
	testGetPendingNode(t, masterAbi, bc, st, 2, common.HexToAddress(normalNodes[1]["address"].(string)), uint64(1))
	testVotePending(t, masterAbi, bc, st, []map[string]interface{}{genesisNodes[0]}, uint64(4))
	testGetPendingNode(t, masterAbi, bc, st, 2, common.HexToAddress(normalNodes[1]["address"].(string)), uint64(2))
	testVotePending(t, masterAbi, bc, st, genesisNodes, uint64(5))

	// test delete latest node
	testRequestDelete(t, masterAbi, bc, st, 5, common.HexToAddress(normalNodes[0]["owner"].(string)), common.HexToAddress(normalNodes[1]["address"].(string)))
	testVoteDeleteNode(t, masterAbi, bc, st, 1, 5, 2, common.HexToAddress(genesisNodes[0]["owner"].(string)), common.HexToAddress(normalNodes[1]["address"].(string)))
	testVoteDeleteNode(t, masterAbi, bc, st, 1, 5, 3, common.HexToAddress(genesisNodes[1]["owner"].(string)), common.HexToAddress(normalNodes[1]["address"].(string)))
	testVoteDeleteNode(t, masterAbi, bc, st, 1, 5, 4, common.HexToAddress(genesisNodes[2]["owner"].(string)), common.HexToAddress(normalNodes[1]["address"].(string)))
	testAvailableNodes(t, masterAbi, bc, st, uint64(4))

	testAddPendingNode(t, masterAbi, bc, st, normalNodes[2], common.HexToAddress(normalNodes[0]["owner"].(string)))
	testHasPendingVoted(t, masterAbi, bc, st, common.HexToAddress(genesisNodes[0]["owner"].(string)), uint64(3), false)
	testGetPendingNode(t, masterAbi, bc, st, 3, common.HexToAddress(normalNodes[2]["address"].(string)), uint64(1))
	testVotePending(t, masterAbi, bc, st, []map[string]interface{}{genesisNodes[0]}, uint64(4))
	testGetPendingNode(t, masterAbi, bc, st, 3, common.HexToAddress(normalNodes[2]["address"].(string)), uint64(2))
	testVotePending(t, masterAbi, bc, st, genesisNodes, uint64(5))

	// test withdraw: assume staker withdraw an amount of KAI.
	withdraw, _ := big.NewInt(0).SetString("500000000000000000", 10)
	testGetAvailableNodeIndex(t, masterAbi, bc, st, common.HexToAddress(genesisNodes[0]["address"].(string)), uint64(2))
	testWithdraw(t, masterAbi, bc, st, common.HexToAddress(genesisNodes[0]["address"].(string)), common.HexToAddress(genesisNodes[0]["staker"].(string)), withdraw, 4)
	testAvailableNodes(t, masterAbi, bc, st, uint64(5))
}

func TestNode(t *testing.T) {
	kAbi, err := abi.JSON(strings.NewReader(NodeAbi))
	require.NoError(t, err)

	input, err := kAbi.Pack("",
		masterAddress,
		common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"),
		"7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860",
		"node1",
		"127.0.0.1",
		"3000",
		uint16(500),
	)
	require.NoError(t, err)

	newCode := append(NodeByteCode, input...)
	bc, err := setupBlockchain()
	if err != nil {
		t.Fatal(err)
	}
	st, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x0000000000000000000000000000000000000010")

	// Setup contract code into genesis state
	_, contractAddr, _, err := create(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), address, bc.CurrentHeader(), bc, newCode, big.NewInt(0), st)
	require.NoError(t, err)
	require.Equal(t, address, *contractAddr)

	getOwner, err := kAbi.Pack("getOwner")
	require.NoError(t, err)

	result, err := staticCall(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), *contractAddr, bc.CurrentHeader(), bc, getOwner, st)
	require.NoError(t, err)

	// test get owner
	owner := common.BytesToAddress(result)
	require.Equal(t, "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6", owner.Hex())

	// test get node info
	type nodeInfo struct {
		Owner common.Address `abi:"owner"`
		NodeId string `abi:"nodeId"`
		NodeName string `abi:"nodeName"`
		IpAddress string `abi:"ipAddress"`
		Port string `abi:"port"`
		RewardPercentage uint16 `abi:"rewardPercentage"`
		Balance *big.Int `abi:"balance"`
	}
	getNodeInfo, err := kAbi.Pack("getNodeInfo")
	result, err = staticCall(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), *contractAddr, bc.CurrentHeader(), bc, getNodeInfo, st)
	require.NoError(t, err)

	var actualNodeInfo nodeInfo
	err = kAbi.Unpack(&actualNodeInfo, "getNodeInfo", result)
	require.NoError(t, err)

	expectedNodeInfo := &nodeInfo{
		Owner:            common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"),
		NodeId:           "7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860",
		NodeName:         "node1",
		IpAddress:        "127.0.0.1",
		Port:             "3000",
		RewardPercentage: uint16(500),
		Balance:          big.NewInt(0),
	}
	require.Equal(t, expectedNodeInfo.Owner, actualNodeInfo.Owner)
	require.Equal(t, expectedNodeInfo.NodeId, actualNodeInfo.NodeId)
}
