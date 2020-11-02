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

package tests

import (
	"crypto/ecdsa"
	"encoding/hex"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	"github.com/kardiachain/go-kardiamain/mainchain/permissioned"
	"github.com/kardiachain/go-kardiamain/types"
)

// getPrivateKeyToCallSmc returns private key of account generated in GetBlockchain() for testing purpose
func getPrivateKeyToCallSmc() *ecdsa.PrivateKey {
	addrKeyBytes, _ := hex.DecodeString("8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06")
	addrKey := crypto.ToECDSAUnsafe(addrKeyBytes)
	return addrKey
}

func GetCandidateSmcUtil() (*permissioned.CandidateSmcUtil, error) {
	bc, err := GetBlockchain()
	if err != nil {
		return nil, err
	}
	util, err := permissioned.NewCandidateSmcUtil(bc, getPrivateKeyToCallSmc())
	if err != nil {
		return nil, err
	}
	return util, nil
}

// ApplyTransactionReturnLog applies an tx to a blockchain and returns all the logs generated from that tx
func ApplyTransactionReturnLog(bc base.BaseBlockChain, statedb *state.StateDB, tx *types.Transaction) ([]*types.Log, error) {
	var (
		usedGas = new(uint64)
		header  = &types.Header{Time: time.Now(), GasLimit: 10000000}
		gp      = new(types.GasPool).AddGas(10000000)
		logger  = log.New()
	)
	statedb.Prepare(tx.Hash(), common.Hash{}, 1)
	receipt, _, err := blockchain.ApplyTransaction(logger, bc, gp, statedb, header, tx, usedGas, kvm.Config{})

	if err != nil {
		return nil, err
	}
	return receipt.Logs, nil
}
