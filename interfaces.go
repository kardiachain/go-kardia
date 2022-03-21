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
	"context"
	"errors"
	"math/big"

	"github.com/kardiachain/go-kardia/lib/common"
)

// NotFound is returned by API methods if the requested item does not exist.
var NotFound = errors.New("not found")

// CallMsg contains parameters for contract calls.
type CallMsg struct {
	From     common.Address  // the sender of the 'transaction'
	To       *common.Address // the destination contract (nil for contract creation)
	Gas      uint64          // if 0, the call executes with near-infinite gas
	GasPrice *big.Int        // wei <-> gas exchange ratio
	Value    *big.Int        // amount of wei sent along with the call
	Data     []byte          // input data, usually an ABI-encoded contract method invocation
}

// FilterQuery contains options for contract log filtering.
type FilterQuery struct {
	BlockHash *common.Hash     // used by kai_getLogs, return logs only from block with this hash
	FromBlock uint64           // beginning of the queried range, nil means genesis block
	ToBlock   uint64           // end of the range, nil means latest block
	Addresses []common.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position AND B in second position
	// {{A}, {B}}         matches topic A in first position AND B in second position
	// {{A, B}, {C, D}}   matches topic (A OR B) in first position AND (C OR D) in second position
	Topics [][]common.Hash
}

// ChainStateReader wraps access to the state trie of the canonical blockchain. Note that
// implementations of the interface may be unable to return state values for old blocks.
// In many cases, using CallContract can be preferable to reading raw contract storage.
type ChainStateReader interface {
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
}
