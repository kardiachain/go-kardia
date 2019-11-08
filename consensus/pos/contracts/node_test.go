package contracts

import (
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"
)



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
	_, contractAddr, _, err := create(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), address, bc.CurrentHeader(), bc, newCode, st)
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


