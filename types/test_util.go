package types

import (
	"time"

	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

func MakeCommit(blockID BlockID, height uint64, round uint32,
	voteSet *VoteSet, validators []PrivValidator, now time.Time) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		addr := validators[i].GetAddress()
		vote := &Vote{
			ValidatorAddress: addr,
			ValidatorIndex:   uint32(i),
			Height:           height,
			Round:            round,
			Type:             kproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err := signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(voteSet.ChainID(), v)
	if err != nil {
		return false, err
	}
	vote.Signature = v.Signature
	return voteSet.AddVote(vote)
}
