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

package kardia

import (
	"crypto/ecdsa"
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardia/dev"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
)

// The following function is just call the master smc and return result in bytes format
func CallStaticKardiaMasterSmc(from common.Address, to common.Address, bc *blockchain.BlockChain, input []byte, statedb *state.StateDB) (result []byte, err error) {
	context := blockchain.NewKVMContextFromDualNodeCall(from, bc.CurrentHeader(), bc)
	vmenv := kvm.NewKVM(context, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(100000))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

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
