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

package blockchain

import (
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/kai/base"
)

// Manages the internal blockchain (i.e Kardia) and one of the external blockchain (e.g. Ethereum,
// Neo, etc.). Provides all necessary methods to interact with either one.
type DualBlockChainManager struct {
	externalBlockChain base.BlockChainAdapter
	internalBlockChain base.BlockChainAdapter
}

func NewDualBlockChainManager(internal base.BlockChainAdapter, external base.BlockChainAdapter) *DualBlockChainManager {
	return &DualBlockChainManager{
		internalBlockChain: internal,
		externalBlockChain: external,
	}
}

func (d *DualBlockChainManager) SubmitTx(event *types.EventData) error {
	if event.FromExternal {
		return d.internalBlockChain.SubmitTx(event)
	}

	return d.externalBlockChain.SubmitTx(event)
}
