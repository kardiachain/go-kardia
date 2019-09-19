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

package service

import (
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/types"
)

type DualConfig struct {
	// Protocol options
	NetworkId uint64 // Network

	ChainID uint64 //Chain id unique to dual node group, such as group connecting to Eth.

	// The genesis block of dual blockchain, which is inserted if the database is empty.
	// If nil, the Dual main net block is used.
	DualGenesis *genesis.Genesis `toml:",omitempty"`

	// Dual's event pool options
	DualEventPool event_pool.Config

	// DbInfo stores configuration information to setup database
	DBInfo storage.DbInfo

	// isPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	IsPrivate bool

	ProtocolName string

	// BaseAccount defines account which is used to execute internal smart contracts
	BaseAccount *types.BaseAccount
}
