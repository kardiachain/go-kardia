/*
 *  Copyright 2021 KardiaChain
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

package misc

import (
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/configs/stakingV3"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/common"
)

// ApplyGalaxiasContracts modifies the state database according to the Galaxias Mainnet hardfork rules:
// - Apply new staking, params, treasury contracts' bytecode
func ApplyStakingV3Contracts(statedb *state.StateDB, valsList []common.Address) {
	statedb.SetCode(configs.StakingContractAddress, common.FromHex(stakingV3.StakingV3StakingSMCBytecode))
	for i := range valsList {
		statedb.SetCode(valsList[i], common.FromHex(stakingV3.StakingV3ValidatorSMCBytecode))
	}
}