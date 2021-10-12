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
	"math/big"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// GetHeader returns the hash corresponding to their hash.
	GetHeader(common.Hash, uint64) *types.Header
}

// NewKVMContext creates a new context for use in the KVM.
func NewKVMContext(msg types.Message, header *types.Header, chain ChainContext) kvm.Context {
	return kvm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Coinbase:    header.ProposerAddress,
		BlockHeight: new(big.Int).SetUint64(header.Height),
		Time:        new(big.Int).SetUint64(uint64(header.Time.Unix())),
		GasLimit:    header.GasLimit,
	}
}

// NewKVMContext creates a new context for dual node to call smc in the KVM.
func NewKVMContextFromDualNodeCall(from common.Address, header *types.Header, chain ChainContext) kvm.Context {
	return kvm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Coinbase:    header.ProposerAddress,
		BlockHeight: new(big.Int).SetUint64(header.Height),
		Time:        new(big.Int).SetUint64(uint64(header.Time.Unix())),
		GasLimit:    header.GasLimit,
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by height
func GetHashFn(ref *types.Header, chain ChainContext) func(n uint64) common.Hash {
	var cache []common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if len(cache) == 0 {
			cache = append(cache, ref.LastCommitHash)
		}
		if idx := ref.Height - n - 1; idx < uint64(len(cache)) {
			return cache[idx]
		}
		// No luck in the cache, but we can start iterating from the last element we already know
		lastKnownHash := cache[len(cache)-1]
		lastKnownHeight := ref.Height - uint64(len(cache))

		// Not cached, iterate the blocks and cache the hashes
		for {
			header := chain.GetHeader(lastKnownHash, lastKnownHeight)
			if header == nil {
				break
			}
			cache = append(cache, header.LastCommitHash)
			lastKnownHash = header.LastCommitHash
			lastKnownHeight = header.Height - 1
			if n == lastKnownHeight {
				return lastKnownHash
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks wether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db kvm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db kvm.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}
