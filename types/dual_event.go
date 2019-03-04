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
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

type BlockchainSymbol string

// Enum for
const (
	KARDIA   = BlockchainSymbol("KAI")
	ETHEREUM = BlockchainSymbol("ETH")
	NEO      = BlockchainSymbol("Neo")
)

// An event pertaining to the current dual node's interests and its derived tx's
// metadata.
type DualEvent struct {
	Nonce             uint64      `json:"nonce"  	 			 gencodec:"required"`
	TriggeredEvent    *EventData  `json:"triggeredEvent"		 gencodec:"required"`
	PendingTxMetadata *TxMetadata `json:"pendingTxMetadata"      gencodec:"required"`

	KardiaSmcs []*KardiaSmartcontract `json:"kardiaSmcs"				 gencodec:"required"`

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type KardiaSmartcontract struct {
	EventWatcher *Watcher
	Actions      *DualActions
}

type Watcher struct {
	// Use string type because since different blockchain may have its own address type and string
	// is a universal type.
	SmcAddress string
}

type DualActions struct {
	Actions []*DualAction
}

type DualAction struct {
	Name string
}

// Data relevant to the event (either from external or internal blockchain)
// that pertains to the current dual node's interests.
type EventData struct {
	TxHash       common.Hash
	TxSource     BlockchainSymbol
	FromExternal bool
	Data         *EventSummary

	// caches
	hash atomic.Value
}

func (ed *EventData) String() string {
	return fmt.Sprintf("EventData{TxHash:%v  TxSource:%v  FromExternal:%v  Data:%v}",
		ed.TxHash.Hex(),
		ed.TxSource,
		ed.FromExternal,
		ed.Data)

}

// Hash returns a hash from an EventData object
func (ev *EventData) Hash() common.Hash {
	if hash := ev.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(ev)
	ev.hash.Store(v)
	return v
}

// Relevant bits for necessary for computing internal tx (ie. Kardia's tx)
// or external tx (ie. Ether's tx, Neo's tx).
type EventSummary struct {
	TxMethod string   // Smc's method
	TxValue  *big.Int // Amount of the tx
	ExtData  [][]byte // Additional data along with this event
}

// String returns a string representation of EventSummary
func (eventSummary *EventSummary) String() string {
	return fmt.Sprintf("Data{TxMethod:%v  TxValue:%v}",
		eventSummary.TxMethod, eventSummary.TxValue)
}

// Metadata relevant to the tx that will be submit to other blockchain (internally
// or externally).
type TxMetadata struct {
	TxHash common.Hash
	Target BlockchainSymbol
}

// String returns a string representation of TxMetadata
func (txMetadata *TxMetadata) String() string {
	return fmt.Sprintf("TxMetadata{TxHash:%v  Target:%v}",
		txMetadata.TxHash.Hex(), txMetadata.Target)
}

func NewDualEvent(nonce uint64, fromExternal bool, txSource BlockchainSymbol, txHash *common.Hash, summary *EventSummary) *DualEvent {
	return &DualEvent{
		Nonce: nonce,
		TriggeredEvent: &EventData{
			TxHash:       *txHash,
			TxSource:     txSource,
			FromExternal: fromExternal,
			Data:         summary,
		},
	}
}

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

// Returns a short string representing DualEvent
func (de *DualEvent) String() string {
	if de == nil {
		return "nil-DualEvent"
	}
	return fmt.Sprintf("DualEvent{Nonce:%v  TriggeredEvent:%v}#%v",
		de.Nonce,
		de.TriggeredEvent,
		de.Hash().Fingerprint())
}

// Transactions is a Transaction slice type for basic sorting.
type DualEvents []*DualEvent

// Len returns the length of s.
func (d DualEvents) Len() int { return len(d) }

// GetRlp implements Rlpable and returns the i'th element of d in rlp.
func (d DualEvents) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(d[i])
	return enc
}

// DualEventByNonce implements the sort interface to allow sorting a list of dual's events
// by their nonces.
type DualEventByNonce DualEvents

func (d DualEventByNonce) Len() int           { return len(d) }
func (d DualEventByNonce) Less(i, j int) bool { return d[i].Nonce < d[j].Nonce }
func (d DualEventByNonce) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
