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

package tx_pool

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool, legacy bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = configs.TxGasContractCreation
	} else {
		gas = configs.TxGas
		if legacy {
			gas = configs.TxGasLegacy
		}
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		nonZeroGas := configs.TxDataNonZeroGas
		if (math.MaxUint64-gas)/nonZeroGas < nz {
			return 0, kvm.ErrGasUintOverflow
		}
		gas += nz * nonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/configs.TxDataZeroGas < z {
			return 0, kvm.ErrGasUintOverflow
		}
		gas += z * configs.TxDataZeroGas
	}
	return gas, nil
}

func GenerateSmcCall(senderKey *ecdsa.PrivateKey, address common.Address, input []byte, txPool *TxPool, isIncrement bool) *types.Transaction {
	senderAddress := crypto.PubkeyToAddress(senderKey.PublicKey)
	nonce := txPool.Nonce(senderAddress)
	if isIncrement {
		nonce++
	}
	tx, err := types.SignTx(
		types.HomesteadSigner{},
		types.NewTransaction(
			nonce,
			address,
			big.NewInt(0),
			5000000,
			big.NewInt(1),
			input,
		), senderKey)
	if err != nil {
		panic(fmt.Sprintf("Fail to generate smc call: %v", err))
	}
	log.Error("GenerateSmcCall", "nonce", tx.Nonce(), "tx", tx.Hash().Hex())
	return tx
}
