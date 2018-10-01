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
	"github.com/kardiachain/go-kardia/blockchain"
)

// DefaultConfig contains default settings for use on the Kardia main net.
var DefaultConfig = Config{

	NetworkId: 1,

	TxPool: blockchain.DefaultTxPoolConfig,
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// Protocol options
	NetworkId uint64 // Network

	// The genesis block, which is inserted if the database is empty.
	// If nil, the Kardia main net block is used.
	Genesis *blockchain.Genesis `toml:",omitempty"`

	// Transaction pool options
	TxPool blockchain.TxPoolConfig

	// chaindata
	ChainData string

	// DB caches
	DbCaches int

	// DB handles
	DbHandles int

	// acceptTxs accept tx sync processes
	AcceptTxs uint32
}
