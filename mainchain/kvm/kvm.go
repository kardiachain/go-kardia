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

package kvm

import (
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/pos"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"math/big"

	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/types"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// GetHeader returns the hash corresponding to their hash.
	CurrentHeader() *types.Header
	GetHeader(common.Hash, uint64) *types.Header
	Config() *types.ChainConfig
	GetBlockByHeight(height uint64) *types.Block
	CurrentBlock() *types.Block
	ZeroFee() bool
	GetBlockReward() *big.Int
	GetConsensusMasterSmartContract() pos.MasterSmartContract
	ApplyMessage(vm *kvm.KVM, msg types.Message, gp *types.GasPool) ([]byte, uint64, bool, error)
	State() (*state.StateDB, error)
}

// StateDB is an KVM database for full state querying.
type StateDB interface {
	CreateAccount(common.Address)

	AddBalance(common.Address, *big.Int)
	SubBalance(common.Address, *big.Int)
	GetBalance(common.Address) *big.Int

	GetCodeHash(common.Address) common.Hash
	GetCode(common.Address) []byte
	SetCode(common.Address, []byte)
	GetCodeSize(common.Address) int

	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash)

	GetNonce(common.Address) uint64
	SetNonce(common.Address, uint64)

	AddRefund(uint64)
	SubRefund(uint64)
	GetRefund() uint64

	Suicide(common.Address) bool
	HasSuicided(common.Address) bool

	RevertToSnapshot(int)
	Snapshot() int

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided accounts.
	Exist(common.Address) bool

	// Empty returns whether the given account is empty. Empty
	// is defined as (balance = nonce = code = 0).
	Empty(common.Address) bool

	AddLog(*types.Log)
	AddPreimage(common.Hash, []byte)
}

// NewKVMContext creates a new context for use in the KVM.
func NewKVMContext(msg types.Message, header *types.Header, chain base.BaseBlockChain) kvm.Context {
	return kvm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Origin:      msg.From(),
		BlockHeight: new(big.Int).SetUint64(header.Height),
		Time:        new(big.Int).Set(header.Time),
		GasLimit:    header.GasLimit,
		GasPrice:    new(big.Int).Set(msg.GasPrice()),
		Chain: chain,
	}
}

// NewKVMContext creates a new context for dual node to call smc in the KVM.
func NewKVMContextFromDualNodeCall(from common.Address, header *types.Header, chain base.BaseBlockChain) kvm.Context {
	return kvm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Origin:      from,
		BlockHeight: new(big.Int).SetUint64(header.Height),
		Time:        new(big.Int).Set(header.Time),
		GasLimit:    header.GasLimit,
		GasPrice:    big.NewInt(1),
		Chain: chain,
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by height
func GetHashFn(ref *types.Header, chain base.BaseBlockChain) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]common.Hash{
				ref.Height - 1: ref.LastCommitHash,
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.LastCommitHash, ref.Height-1); header != nil; header = chain.GetHeader(header.LastCommitHash, header.Height-1) {
			cache[header.Height-1] = header.LastCommitHash
			if n == header.Height-1 {
				return header.LastCommitHash
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks wether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db base.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db base.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}
