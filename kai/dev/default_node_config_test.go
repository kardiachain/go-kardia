package dev

import (
	"testing"
	"os"
	cmn "github.com/kardiachain/go-kardia/lib/common"
)

var votingStrategyTests = []struct {
	Height		*cmn.BigInt
	Round   	*cmn.BigInt
	VoteType 	byte
	Result 		*cmn.BigInt
}{
	{cmn.NewBigInt(1), cmn.NewBigInt(0),1, cmn.NewBigInt(0)},
	{cmn.NewBigInt(1), cmn.NewBigInt(0),2, cmn.NewBigInt(1)},
	{cmn.NewBigInt(1), cmn.NewBigInt(0),1, cmn.NewBigInt(-1)},
}

func TestDevEnvironmentConfig_SetVotingStrategy(t *testing.T) {
	os.Chdir("../..")
	devEnv := CreateDevEnvironmentConfig()
	devEnv.SetVotingStrategy("voting_strategy.csv")
	for i, test := range votingStrategyTests {
		votingStrategy := devEnv.VotingStrategy[i]
		height := test.Height
		round := test.Round
		voteType := test.VoteType
		result := test.Result


		if !height.Equals(votingStrategy.Height) {
			t.Errorf("Expected height %v got %v", height, votingStrategy.Height)
		}
		if !round.Equals(votingStrategy.Round) {
			t.Errorf("Expected round %v got %v", round, votingStrategy.Round)
		}

		if voteType != votingStrategy.VoteType {
			t.Errorf("Expected round %v got %v", voteType, votingStrategy.VoteType)
		}

		if !result.Equals(votingStrategy.Result) {
			t.Errorf("Expected result %v got %v", result, votingStrategy.Result)
		}

	}
}

func TestDevEnvironmentConfig_DecideVoteStrategy(t *testing.T) {
	os.Chdir("../..")
	devEnv := CreateDevEnvironmentConfig()
	devEnv.SetVotingStrategy("voting_strategy.csv")
	for i, test := range votingStrategyTests {
		votingStrategy := devEnv.VotingStrategy[i]
		var result = devEnv.DecideVoteStrategy(test.Height,test.Round, test.VoteType)
		if (result != votingStrategy.Result.Int32()) {
			t.Errorf("Expected result %v got %v", result, votingStrategy.Result.Int32)
		}

	}

}