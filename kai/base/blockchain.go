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
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/types"
)

type BaseBlockChain interface {
	Genesis() *types.Block
	CurrentHeader() *types.Header
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	GetBlockByHeight(height uint64) *types.Block
	GetBlockByHash(hash common.Hash) *types.Block
	State() (*state.StateDB, error)
	CommitTrie(root common.Hash) error
	WriteBlockInfo(block *types.Block, blockInfo *types.BlockInfo)
	ReadCommit(height uint64) *types.Commit
	Config() *configs.ChainConfig
	GetHeader(common.Hash, uint64) *types.Header
	IsPrivate() bool
	HasPermission(peer *p2p.Peer) bool
	SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription
	StateAt(root uint64) (*state.StateDB, error)
	DB() types.StoreDB
	ZeroFee() bool
	ApplyMessage(vm *kvm.KVM, msg types.Message, gp *types.GasPool) (*kvm.ExecutionResult, error)
}
