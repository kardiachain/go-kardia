/*
 *  Copyright 2020 KardiaChain
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

package configs

import (
	"fmt"
	"math/big"
)

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	ChainID       *big.Int `json:"chainId,omitempty" yaml:"ChainID"`             // chainId identifies the current chain and is used for replay protection
	GalaxiasBlock *uint64  `json:"galaxiasBlock,omitempty" yaml:"galaxiasBlock"` // Mainnet V2 switch block (nil = no fork, 0 = already V2)

	// Various consensus engines
	Kaicon *KaiconConfig `json:"kaicon,omitempty" yaml:"KaiconConfig"`

	ChainID *big.Int `json:"chainId,omitempty" yaml:"ChainID"`
}

// KaiconConfig is the consensus engine configs for Kardia BFT DPoS.
type KaiconConfig struct {
	Period uint64 `json:"period" yaml:"Period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch" yaml:"Epoch"`   // Epoch length to reset votes and checkpoint
}

// String implements the stringer interface, returning the consensus engine details.
func (c *KaiconConfig) String() string {
	return "kaicon"
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	var engine interface{}
	switch {
	case c.Kaicon != nil:
		engine = c.Kaicon
	default:
		engine = "unknown"
	}
	return fmt.Sprintf("{Engine: %v}",
		engine,
	)
}

// IsGalaxias returns the comparison head block height for Galaxias fork
func (c *ChainConfig) IsGalaxias(height *uint64) bool {
	return isForked(c.GalaxiasBlock, height)
}

// isForked returns whether a fork scheduled at block s is active at the given head block.
func isForked(s, head *uint64) bool {
	if s == nil || head == nil {
		return false
	}
	return *s <= *head
}
