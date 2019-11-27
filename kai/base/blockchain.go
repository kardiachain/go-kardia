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

package base

import (
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/pos"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
)

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

// ContractRef is a reference to the contract's backing object
type ContractRef interface {
	Address() common.Address
}

type KVM interface {
	Cancel()
	Cancelled() bool
	IsZeroFee() bool
	Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error)
	DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error)
	StaticCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error)
	Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error)
	CreateGenesisContract(caller ContractRef, contract *common.Address, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error)
	GetStateDB() StateDB
	GetCoinbase() common.Address
}

type BaseBlockChain interface {
	Genesis() *types.Block
	CurrentHeader() *types.Header
	CurrentBlock() *types.Block
	WriteBlockWithoutState(*types.Block, *types.PartSet, *types.Commit) error
	GetBlock(hash common.Hash, number uint64) *types.Block
	GetBlockByHeight(height uint64) *types.Block
	GetBlockByHash(hash common.Hash) *types.Block
	State() (*state.StateDB, error)
	CommitTrie(root common.Hash) error
	WriteReceipts(receipts types.Receipts, block *types.Block)
	ReadCommit(height uint64) *types.Commit
	Config() *types.ChainConfig
	GetHeader(common.Hash, uint64) *types.Header
	SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription
	StateAt(height uint64) (*state.StateDB, error)
	DB() types.StoreDB
	ZeroFee() bool
	ApplyMessage(vm KVM, msg types.Message, gp *types.GasPool) ([]byte, uint64, bool, error)
	GetFetchNewValidatorsTime() uint64
	GetBlockReward() *big.Int
	GetConsensusMasterSmartContract() pos.MasterSmartContract
	GetConsensusNodeAbi() string
	GetConsensusStakerAbi() string
	CheckCommittedStateRoot(root common.Hash) bool
}
