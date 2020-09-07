package types

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p/enode"
	"github.com/kardiachain/go-kardiamain/types"
)

func TestPeerCatchupRounds(t *testing.T) {

	valSet, privSet := types.RandValidatorSet(10, 1)

	logger := log.New()
	logger.AddTag("test vote set")

	var peer enode.ID

	peer32 := []byte("peer1")
	copy(peer[:], peer32)
	hvs := NewHeightVoteSet(logger, "mainchain", common.NewBigInt64(1), valSet)
	vote999_0 := makeVoteHR(t, int64(1), int64(0), int64(999), privSet)
	added, err := hvs.AddVote(vote999_0, peer)

	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1000_0 := makeVoteHR(t, int64(1), int64(0), 1000, privSet)
	added, err = hvs.AddVote(vote1000_0, peer)
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

}

func makeVoteHR(t *testing.T, height int64, valIndex int64, round int64, privVals []*ecdsa.PrivateKey) *types.Vote {
	privVal := types.NewPrivValidator(privVals[valIndex])
	// pubKey := privVal.GetPubKey()

	blockHash := common.BytesToHash(common.RandBytes(32))
	partSetHash := common.BytesToHash(common.RandBytes(32))
	blockPartsHeaders := types.PartSetHeader{Total: uint32(123), Hash: partSetHash}

	vote := &types.Vote{
		ValidatorAddress: privVal.GetAddress(),
		ValidatorIndex:   common.NewBigInt64(valIndex),
		Height:           common.NewBigInt64(height),
		Round:            common.NewBigInt64(round),
		Timestamp:        big.NewInt(time.Now().Unix()),
		Type:             types.VoteTypePrecommit,
		BlockID:          types.BlockID{Hash: blockHash, PartsHeader: blockPartsHeaders},
	}
	chainID := "mainchain"

	err := privVal.SignVote(chainID, vote)
	if err != nil {
		panic(fmt.Sprintf("Error signing vote: %v", err))
	}

	return vote
}
