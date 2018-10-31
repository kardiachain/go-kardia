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

package dual

import (
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	ethState "github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/kardiachain/go-kardia/dualchain/external/eth/ethsmc"
	"github.com/kardiachain/go-kardia/lib/log"
)

func CreateEthReleaseAmountTx(recipientAddr ethCommon.Address, statedb *ethState.StateDB, quantity *big.Int, ethSmc *ethsmc.EthSmc) *ethTypes.Transaction {
	// Nonce of account to sign tx
	nonce := statedb.GetNonce(recipientAddr)
	if nonce == 0 {
		log.Error("Eth state return 0 for nonce of contract address, SKIPPING TX CREATION", "addr", ethsmc.EthContractAddress)
	}
	tx := ethSmc.CreateEthReleaseTx(quantity, nonce)
	log.Info("Create Eth tx to release", "quantity", quantity, "nonce", nonce, "txhash", tx.Hash().Hex())

	return tx
}
