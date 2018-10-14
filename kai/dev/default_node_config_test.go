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

package dev

import (
	"testing"
)

func TestDevEnvironmentConfig_SetVotingStrategy_GetScriptVote(t *testing.T) {
	var expected_votes = map[VoteTurn]int{
		{2, 0, 1}: -1,
		{4, 0, 1}: -1,
		{4, 0, 2}: -1,
		{5, 0, 1}: -1,
	}

	devEnv := CreateDevEnvironmentConfig()
	devEnv.SetVotingStrategy("voting_scripts/voting_strategy_1.csv")

	for test, result := range expected_votes {
		if devEnv.VotingStrategy[test] != result {
			t.Errorf("Expected result %v got %v", result, devEnv.VotingStrategy[test])
		}

		var r, _ = devEnv.GetScriptedVote(test.Height, test.Round, test.VoteType)

		if r != result {
			t.Errorf("Expected result %v got %v", result, r)
		}
	}
}
