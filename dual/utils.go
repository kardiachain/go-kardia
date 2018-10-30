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
	"crypto/ecdsa"
	"math/big"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	ethState "github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/kardiachain/go-kardia/abi"
	"github.com/kardiachain/go-kardia/dual/ethsmc"
	"github.com/kardiachain/go-kardia/kai/dev"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
)

// Creates a Kardia tx to report new matching amount from Eth/Neo network.
// type = 1: ETH
// type = 2: NEO
// TODO(namdoh@): Make type of matchType an enum instead of an int.
func CreateKardiaMatchAmountTx(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity *big.Int, matchType int) *types.Transaction {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	kABI, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var getAmountToSend []byte
	if matchType == 1 {
		getAmountToSend, err = kABI.Pack("matchEth", quantity)
	} else {
		getAmountToSend, err = kABI.Pack("matchNeo", quantity)
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr, "dual", "dual")

	}
	return tool.GenerateSmcCall(senderKey, masterSmcAddr, getAmountToSend, statedb)
}

// Call to remove amount of ETH / NEO on master smc
// type = 1: ETH
// type = 2: NEO
func CreateKardiaRemoveAmountTx(senderKey *ecdsa.PrivateKey, statedb *state.StateDB, quantity *big.Int, matchType int) *types.Transaction {
	masterSmcAddr := dev.GetContractAddressAt(2)
	masterSmcAbi := dev.GetContractAbiByAddress(masterSmcAddr.String())
	abi, err := abi.JSON(strings.NewReader(masterSmcAbi))

	if err != nil {
		log.Error("Error reading abi", "err", err)
	}
	var amountToRemove []byte
	if matchType == 1 {
		amountToRemove, err = abi.Pack("removeEth", quantity)
	} else {
		amountToRemove, err = abi.Pack("removeNeo", quantity)
		log.Info("byte to send to remove", "byte", string(amountToRemove), "neodual", "neodual")
	}

	if err != nil {
		log.Error("Error getting abi", "error", err, "address", masterSmcAddr, "dual", "dual")

	}
	return tool.GenerateSmcCall(senderKey, masterSmcAddr, amountToRemove, statedb)
}

//
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
