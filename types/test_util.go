package types

import (
	"time"
)

// MakeCommit ...
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
			Type:             VoteTypePrecommit,
			BlockID:          blockID,
			Timestamp:        uint64(now.Unix()),
		}

		_, err := signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	err = privVal.SignVote(voteSet.ChainID(), vote)
	if err != nil {
		return false, err
	}
	return voteSet.AddVote(vote)
}
