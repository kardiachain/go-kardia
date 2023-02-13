package blockchain

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/trie"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bcproto "github.com/kardiachain/go-kardia/proto/kardiachain/blockchain"
	"github.com/kardiachain/go-kardia/types"
)

var TestTx = types.NewTransaction(69, common.Address{69}, new(big.Int).SetInt64(69), 69, new(big.Int).SetInt64(69), []byte("Hello World!"))

func TestBcBlockRequestMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName      string
		requestHeight uint64
		expectErr     bool
	}{
		{"Invalid Request Message", 0, true},
		{"Valid Request Message", 1, false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			request := bcproto.BlockRequest{Height: tc.requestHeight}
			assert.Equal(t, tc.expectErr, ValidateMsg(&request) != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBcNoBlockResponseMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName          string
		nonResponseHeight uint64
		expectErr         bool
	}{
		{"Invalid Non-Response Message", 0, true},
		{"Valid Non-Response Message", 1, false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			nonResponse := bcproto.NoBlockResponse{Height: tc.nonResponseHeight}
			assert.Equal(t, tc.expectErr, ValidateMsg(&nonResponse) != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBcStatusRequestMessageValidateBasic(t *testing.T) {
	request := bcproto.StatusRequest{}
	assert.NoError(t, ValidateMsg(&request))
}

func TestBcStatusResponseMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName       string
		responseHeight uint64
		expectErr      bool
	}{
		{"Valid Response Message", 0, false},
		{"Valid Response Message", 1, false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			response := bcproto.StatusResponse{Height: tc.responseHeight}
			assert.Equal(t, tc.expectErr, ValidateMsg(&response) != nil, "Validate Basic had an unexpected result")
		})
	}
}

// nolint:lll // ignore line length in tests
func TestBlockchainMessageVectors(t *testing.T) {
	block := types.NewBlock(&types.Header{Height: 3}, []*types.Transaction{TestTx}, nil, nil, trie.NewStackTrie(nil))
	//block.Version.Block = 11 // overwrite updated protocol version

	bpb, err := block.ToProto()
	require.NoError(t, err)

	testCases := []struct {
		testName string
		bmsg     proto.Message
		expBytes string
	}{
		{"BlockRequestMessage", &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{Height: 1}}}, "0a020801"},
		{"BlockRequestMessage", &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{Height: math.MaxInt64}}},
			"0a0a08ffffffffffffffff7f"},
		{"BlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_BlockResponse{
			BlockResponse: &bcproto.BlockResponse{Block: bpb}}}, "1a94030a91030ade021803220b088092b8c398feffffff012a460a200000000000000000000000000000000000000000000000000000000000000000122212200000000000000000000000000000000000000000000000000000000000000000322000000000000000000000000000000000000000000000000000000000000000003a20f558b4339cf2863da4f236fafb9bb585b8fc0b8f24959c9d330982fa78fc79ab422000000000000000000000000000000000000000000000000000000000000000004a200000000000000000000000000000000000000000000000000000000000000000522000000000000000000000000000000000000000000000000000000000000000005a2000000000000000000000000000000000000000000000000000000000000000006a20000000000000000000000000000000000000000000000000000000000000000072140000000000000000000000000000000000000000800101122c0a2ae9454545944500000000000000000000000000000000000000458c48656c6c6f20576f726c64218080801a00"},
		{"NoBlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{Height: 1}}}, "12020801"},
		{"NoBlockResponseMessage", &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{Height: math.MaxInt64}}},
			"120a08ffffffffffffffff7f"},
		{"StatusRequestMessage", &bcproto.Message{Sum: &bcproto.Message_StatusRequest{
			StatusRequest: &bcproto.StatusRequest{}}},
			"2200"},
		{"StatusResponseMessage", &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{Height: 1, Base: 2}}},
			"2a0408011002"},
		{"StatusResponseMessage", &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{Height: math.MaxInt64, Base: math.MaxInt64}}},
			"2a1408ffffffffffffffff7f10ffffffffffffffff7f"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			bz, _ := proto.Marshal(tc.bmsg)

			require.Equal(t, tc.expBytes, hex.EncodeToString(bz))
		})
	}
}
