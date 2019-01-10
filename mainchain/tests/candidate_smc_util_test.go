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
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/permissioned"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"testing"
	"time"
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

func TestFindCandidateByEmail(t *testing.T) {
	util, err := GetCandidateSmcUtil()
	if err != nil {
		t.Fatal(err)
	}
	candidateInfo, err := util.GetCandidateByEmail("a@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if candidateInfo.Email != "" {
		t.Error("Expect nil return, got candidate with email", candidateInfo.Email)
	}
}

// ApplyTransactionReturnLog applies an tx to a blockchain and returns all the logs generated from that tx
func ApplyTransactionReturnLog(bc base.BaseBlockChain, statedb *state.StateDB, tx *types.Transaction) ([]*types.Log, error) {
	var (
		usedGas = new(uint64)
		header  = &types.Header{Time: big.NewInt(time.Now().Unix()), GasLimit: 10000000}
		gp      = new(blockchain.GasPool).AddGas(10000000)
		logger  = log.New()
	)
	statedb.Prepare(tx.Hash(), common.Hash{}, 1)
	receipt, _, err := blockchain.ApplyTransaction(logger, bc, gp, statedb, header, tx, usedGas, kvm.Config{})

	if err != nil {
		return nil, err
	}
	return receipt.Logs, nil
}

// Test update info of a candidate internally from a private chain
func TestAddInternalCandidate(t *testing.T) {
	util, err := GetCandidateSmcUtil()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := util.UpdateCandidateInfo("a1", "a1@gmail.com", 20, common.HexToAddress("0xa1"), "internalBC")
	if err != nil {
		t.Fatal(err)
	}
	// Apply newly generated tx
	_, err = ApplyTransactionReturnLog(*util.Bc, util.StateDB, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Get newly added candidate
	candidateInfo, err := util.GetCandidateByEmail("a1@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if candidateInfo.Email != "a1@gmail.com" {
		t.Error("Expect a1@gmail.com, got", candidateInfo.Email)
	}
	if candidateInfo.Addr != common.HexToAddress("0xa1") {
		t.Error("Expect address 0xa1, got ", candidateInfo.Addr.String())
	}
	if candidateInfo.IsExternal {
		t.Error("Expect isExternal is false, got true")
	}
	if candidateInfo.Source != "internalBC" {
		t.Error("Expect source is internalBC, got ", candidateInfo.Source)
	}
	if candidateInfo.Name != "a1" {
		t.Error("Expect name is a1, got ", candidateInfo.Name)
	}
	if candidateInfo.Age.Cmp(big.NewInt(20)) != 0 {
		t.Error("Expect age is 20, got ", candidateInfo.Age.String())
	}
}

// Test update info of a candidate from external private chain data
func TestAddExternalCandidate(t *testing.T) {
	util, err := GetCandidateSmcUtil()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := util.UpdateCandidateInfoFromExternal("a2", "a2@gmail.com", 25, common.HexToAddress("0xa2"), "externalBC")
	if err != nil {
		t.Fatal(err)
	}
	// Apply newly generated tx
	_, err = ApplyTransactionReturnLog(*util.Bc, util.StateDB, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Get newly added candidate
	candidateInfo, err := util.GetCandidateByEmail("a2@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if candidateInfo.Email != "a2@gmail.com" {
		t.Error("Expect a2@gmail.com, got", candidateInfo.Email)
	}
	if candidateInfo.Addr != common.HexToAddress("0xa2") {
		t.Error("Expect address 0xa2, got ", candidateInfo.Addr.String())
	}
	if !candidateInfo.IsExternal {
		t.Error("Expect isExternal is true, got false")
	}
	if candidateInfo.Source != "externalBC" {
		t.Error("Expect source is externalBC, got ", candidateInfo.Source)
	}
	if candidateInfo.Name != "a2" {
		t.Error("Expect name is a2, got ", candidateInfo.Name)
	}
	if candidateInfo.Age.Cmp(big.NewInt(25)) != 0 {
		t.Error("Expect age is 25, got ", candidateInfo.Age.String())
	}
}

// TestRequestCandidateInfoFromExternal test if request info of an external candidate fires correct event
func TestRequestCandidateInfoFromExternal(t *testing.T) {
	util, err := GetCandidateSmcUtil()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := util.RequestCandidateInfo("external@gmail.com", "1", "2")
	logs, err := ApplyTransactionReturnLog(*util.Bc, util.StateDB, tx)
	if err != nil {
		t.Fatal(err)
	}
	// Check if there is event emitted from previous tx
	if len(logs) == 0 {
		t.Error("Expect length of log > 0, 0 is returned")
	}
	type ExternalCandidateInfoRequested struct {
		Email string
		FromOrgId string
		ToOrgId string
	}
	var requestEvent ExternalCandidateInfoRequested
	err = util.Abi.Unpack(&requestEvent, "ExternalCandidateInfoRequested", logs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	// Check if event data is emitted correctly
	if requestEvent.Email != "external@gmail.com" {
		t.Error("Expect request info for external@gmail.com, got ", requestEvent.Email)
	}
	if requestEvent.FromOrgId != "1" {
		t.Error("Expect fromOrgId to be 1, return ", requestEvent.FromOrgId)
	}
	if requestEvent.ToOrgId != "2" {
		t.Error("Expect toOrgId to be 2, return ", requestEvent.FromOrgId)
	}
}
