package contracts

import (
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"
)

func TestStaker(t *testing.T) {
	kAbi, err := abi.JSON(strings.NewReader(StakerAbi))
	require.NoError(t, err)

	input, err := kAbi.Pack("",
		masterAddress,
		common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"),
		big.NewInt(100),
		minimumStakes,
	)
	require.NoError(t, err)

	newCode := append(StakerByteCode, input...)
	bc, err := setupBlockchain()
	if err != nil {
		t.Fatal(err)
	}
	st, err := bc.State()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x0000000000000000000000000000000000000020")

	// Setup contract code into genesis state
	_, contractAddr, _, err := create(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), address, bc.CurrentHeader(), bc, newCode, st)
	require.NoError(t, err)
	require.Equal(t, address, *contractAddr)

	stake, err := kAbi.Pack("stake", common.HexToAddress("0x0000000000000000000000000000000000000010"))
	require.NoError(t, err)

	_, err = call(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), *contractAddr, bc.CurrentHeader(), bc, stake, minimumStakes, st)
	require.NoError(t, err)

	getStakeAmount, err := kAbi.Pack("getStakeAmount", common.HexToAddress("0x0000000000000000000000000000000000000010"))
	result, err := staticCall(common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6"), *contractAddr, bc.CurrentHeader(), bc, getStakeAmount, st)
	require.NoError(t, err)
	type data struct {
		Amount *big.Int
		Valid  bool
	}
	var actualData data
	err = kAbi.Unpack(&actualData, "getStakeAmount", result)
	require.NoError(t, err)

	expectedData := data {
		Amount: minimumStakes,
		Valid: true,
	}
	require.Equal(t, expectedData.Amount.String(), actualData.Amount.String())
	require.Equal(t, expectedData.Valid, actualData.Valid)
}
