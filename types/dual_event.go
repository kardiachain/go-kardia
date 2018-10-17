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

package types

import (
	"math/big"
	"sync/atomic"

	"github.com/kardiachain/go-kardia/lib/common"
)

type DualEvent struct {
	data eventdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type eventdata struct {
	Nonce    uint64       `json:"nonce"    gencodec:"required"`
	TxSource string       `json:"txSource" gencodoc:"required"`
	TxHash   *common.Hash `json:"txHash"   gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func NewDualEvent(nonce uint64, txSource string, txHash *common.Hash) *DualEvent {
	return &DualEvent{
		data: eventdata{
			Nonce:    nonce,
			TxSource: txSource,
			TxHash:   txHash,
		},
	}
}

func (de *DualEvent) Nonce() uint64 { return de.data.Nonce }

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (de *DualEvent) Hash() common.Hash {
	if hash := de.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(de)
	de.hash.Store(v)
	return v
}

// Transactions is a Transaction slice type for basic sorting.
type DualEvents []*DualEvent

// Len returns the length of s.
func (d DualEvents) Len() int { return len(d) }

// DualEventByNonce implements the sort interface to allow sorting a list of dual's events
// by their nonces.
type DualEventByNonce DualEvents

func (d DualEventByNonce) Len() int           { return len(d) }
func (d DualEventByNonce) Less(i, j int) bool { return d[i].data.Nonce < d[j].data.Nonce }
func (d DualEventByNonce) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
