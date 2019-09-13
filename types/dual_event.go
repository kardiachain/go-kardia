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
	"crypto/ecdsa"
	"fmt"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"math/big"
	"sync/atomic"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

type BlockchainSymbol string

// Enum for
const (
	KARDIA   = BlockchainSymbol("KAI")
)

// An event pertaining to the current dual node's interests and its derived tx's
// metadata.
type DualEvent struct {
	BlockNumber        uint64     `json:"blockNumber"            gencodec:"required"`
	TriggeredEvent    *EventData  `json:"triggeredEvent"         gencodec:"required"`
	PendingTxMetadata *TxMetadata `json:"pendingTxMetadata"      gencodec:"required"`

	// The smart contract info being submitted externally.
	KardiaSmcs []*KardiaSmartcontract `json:"kardiaSmcs"         gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type KardiaSmartcontract struct {
	// Use string type because since different blockchain may have its own address type and string
	// is a universal type.
	SmcAddress string

	// abi of smcAddress
	SmcAbi string

	WatcherActions WatcherActions
	DualActions DualActions
}

type DualActions []*DualAction

type DualAction struct {
	Name string
}

type WatcherActions []*WatcherAction

// WatcherAction bases on method name, new event with correspond dual action name will be submitted to internal/external proxy
type WatcherAction struct {
	Method string
	DualAction string
}

// Data relevant to the event (either from external or internal blockchain)
// that pertains to the current dual node's interests.
type EventData struct {
	TxHash       common.Hash        `json:"txHash"    gencodec:"required"`
	TxSource     BlockchainSymbol   `json:"source"    gencodec:"required"`
	FromExternal bool               `json:"fromExternal" gencodec:"required"`
	Data         *EventSummary      `json:"data"         gencodec:"data"`

	// Actions is temporarily cached to store a list of actions that will be executed upon once
	// the dual event is executed.
	Action *DualAction              `json:"action"      gencodec:"required"`

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

// String returns a string representation of KardiaSmartcontract
func (kardiaSmc *KardiaSmartcontract) String() string {
	if kardiaSmc != nil {
		return fmt.Sprintf("Smc{Addr:%v WatcherActions:%v DualActions:%v}", kardiaSmc.SmcAddress, kardiaSmc.WatcherActions, kardiaSmc.DualActions)
	}
	return "Smc{Addr:nil}"
}

func NewDualEvent(blockNumber uint64, fromExternal bool, txSource BlockchainSymbol, txHash *common.Hash, summary *EventSummary, action *DualAction) *DualEvent {
	return &DualEvent{
		BlockNumber: blockNumber,
		TriggeredEvent: &EventData{
			TxHash:       *txHash,
			TxSource:     txSource,
			FromExternal: fromExternal,
			Data:         summary,
			Action:       action,
		},
		V: new(big.Int),
		R: new(big.Int),
		S: new(big.Int),
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
	return fmt.Sprintf("DualEvent{BlockNumber:%v, TriggeredEvent:%v, Smc:%v}#%v",
		de.BlockNumber,
		de.TriggeredEvent,
		de.KardiaSmcs,
		de.Hash().Fingerprint())
}

// DualEvents is a DualEvent slice type for basic sorting.
type DualEvents []*DualEvent

// Len returns the length of s.
func (d DualEvents) Len() int { return len(d) }

// GetRlp implements Rlpable and returns the i'th element of d in rlp.
func (d DualEvents) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(d[i])
	return enc
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (de *DualEvent) WithSignature(sig []byte) (*DualEvent, error) {
	r, s, v, err := EventSignatureValues(sig)
	if err != nil {
		return nil, err
	}
	cpy := &DualEvent{
		BlockNumber:       de.BlockNumber,
		TriggeredEvent:    de.TriggeredEvent,
		PendingTxMetadata: de.PendingTxMetadata,
		R: r,
		S: s,
		V: v,
	}
	return cpy, nil
}

// SignEvent signs the event using the given signer and private key
func SignEvent(de *DualEvent, prv *ecdsa.PrivateKey) (*DualEvent, error) {
	h := sigEventHash(de)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return de.WithSignature(sig)
}

// sigHash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func sigEventHash(de *DualEvent) common.Hash {
	return rlpHash([]interface{}{
		de.BlockNumber,
		de.TriggeredEvent,
		de.PendingTxMetadata,
		de.KardiaSmcs,
	})
}

// EventSender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// EventSender may cache the address, allowing it to be used regardless of
// signing method.
func EventSender(de *DualEvent) (common.Address, error) {
	if sc := de.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		return sigCache.from, nil
	}

	addr, err := recoverPlain(sigEventHash(de), de.R, de.S, de.V)
	if err != nil {
		return common.Address{}, err
	}
	de.from.Store(sigCache{from: addr})
	return addr, nil
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func EventSignatureValues(sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}
