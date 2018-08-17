package dev

import (
	"testing"
)

func TestDevEnvironmentConfig_SetVotingStrategy_GetScriptVote(t *testing.T) {
	var expected_votes = map[VoteTurn]int {
		{1,0,1}: 0,
		{1,0,2}: 1,
		{1,1,2}: -1,
	}

	devEnv := CreateDevEnvironmentConfig()
	devEnv.SetVotingStrategy("voting_scripts/voting_strategy.csv")

	for test, result := range expected_votes {
		if (devEnv.VotingStrategy[test] != result) {
			t.Errorf("Expected result %v got %v", result, devEnv.VotingStrategy[test])
		}

		var r, _ = devEnv.GetScriptedVote(test.Height, test.Round, test.VoteType)

		if r != result {
			t.Errorf("Expected result %v got %v", result, r)
		}
	}
}
