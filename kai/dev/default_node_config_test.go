package dev

import (
	"testing"
)

var votingStrategyTests = map[int]VotingStrategy{
	0: {1,0,byte(1),0 },
	1: {1,0,byte(2),1 },
	2: {1,1,byte(2),-1},
}

func TestDevEnvironmentConfig_SetVotingStrategy(t *testing.T) {
	devEnv := CreateDevEnvironmentConfig()
	devEnv.SetVotingStrategy("voting_scripts/voting_strategy.csv")
	for i, test := range votingStrategyTests {
		votingStrategy := devEnv.VotingStrategy[i]
		height := test.Height
		round := test.Round
		voteType := test.VoteType
		result := test.Result

		if height != votingStrategy.Height {
			t.Errorf("Expected height %v got %v", height, votingStrategy.Height)
		}

		if round != votingStrategy.Round {
			t.Errorf("Expected round %v got %v", round, votingStrategy.Round)
		}

		if voteType != votingStrategy.VoteType {
			t.Errorf("Expected voteType %v got %v", voteType, votingStrategy.VoteType)
		}

		if result != votingStrategy.Result {
			t.Errorf("Expected result %v got %v", result, votingStrategy.Result)
		}
	}
}

func TestDevEnvironmentConfig_GetScriptedVote(t *testing.T) {
	devEnv := CreateDevEnvironmentConfig()
	devEnv.SetVotingStrategy("voting_scripts/voting_strategy.csv")
	for i, test := range votingStrategyTests {
		votingStrategy := devEnv.VotingStrategy[i]

		var result = devEnv.GetScriptedVote(test.Height, test.Round, test.VoteType)

		if result != votingStrategy.Result {
			t.Errorf("Expected result %v got %v", votingStrategy.Result, result)
		}
	}
}