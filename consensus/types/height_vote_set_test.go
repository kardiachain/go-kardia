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

package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
)

func TestPeerCatchupRounds(t *testing.T) {

	valSet, privSet := types.RandValidatorSet(4, 1)

	logger := log.New()
	logger.AddTag("test vote set")

	var peer p2p.ID

	peer32 := p2p.ID("peer1")
	peer = peer32
	hvs := NewHeightVoteSet(logger, "kaicoin", 1, valSet)
	vote999_0 := makeVoteHR(t, 1, 0, 999, privSet)
	added, err := hvs.AddVote(vote999_0, peer)

	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1000_0 := makeVoteHR(t, 1, 0, 1000, privSet)
	added, err = hvs.AddVote(vote1000_0, peer)
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

}

func makeVoteHR(t *testing.T, height uint64, valIndex uint32, round uint32, privVals []types.PrivValidator) *types.Vote {
	privVal := privVals[valIndex]
	// pubKey := privVal.GetPubKey()

	blockHash := common.BytesToHash(common.RandBytes(32))
	partSetHash := common.BytesToHash(common.RandBytes(32))
	blockPartsHeaders := types.PartSetHeader{Total: uint32(123), Hash: partSetHash}

	vote := &types.Vote{
		ValidatorAddress: privVal.GetAddress(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Timestamp:        time.Now(),
		Type:             kproto.PrecommitType,
		BlockID:          types.BlockID{Hash: blockHash, PartsHeader: blockPartsHeaders},
	}
	chainID := "kaicoin"
	kv := vote.ToProto()
	err := privVal.SignVote(chainID, kv)
	if err != nil {
		panic(fmt.Sprintf("Error signing vote: %v", err))
	}
	vote.Signature = kv.Signature
	return vote
}
