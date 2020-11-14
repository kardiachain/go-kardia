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

package kai

import (
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/storage"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
)

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// Protocol options
	NetworkId uint64 // Network

	ChainId uint64

	// The genesis block, which is inserted if the database is empty.
	// If nil, the Kardia main net block is used.
	Genesis *genesis.Genesis `toml:",omitempty"`

	// Transaction pool options
	TxPool tx_pool.TxPoolConfig

	// DbInfo stores configuration information to setup database
	DBInfo storage.DbInfo

	// acceptTxs accept tx sync processes
	AcceptTxs uint32

	// IsZeroFee is true then sender will be refunded all gas spent for a transaction
	IsZeroFee bool

	// isPrivate is true then peerId will be checked through smc to make sure that it has permission to access the chain
	IsPrivate bool

	// ServiceName is used to display as log's prefix
	ServiceName string

	// Consensus defines the configuration for the Kardia consensus service,
	// including timeouts and details about the block structure.
	Consensus *configs.ConsensusConfig
}
