/*
 *  Copyright 2021 KardiaChain
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
	"errors"
	"fmt"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/trie"
	"github.com/kardiachain/go-kardia/types"
)

// stateAtBlock retrieves the state database associated with a certain block.
// If no state is locally available for the given block, a height of blocks
// are attempted to be reexecuted to generate the desired state. The optional
// base layer statedb can be passed then it's regarded as the statedb of the
// parent block.
func (k *KardiaService) stateAtBlock(block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (statedb *state.StateDB, err error) {
	var (
		current  *types.Block
		database state.Database
		report   = true
		origin   = block.Height()
	)
	// Check the live database first if we have the state fully available, use that.
	if checkLive {
		statedb, err = k.blockchain.StateAt(block.Height())
		if err == nil {
			return statedb, nil
		}
	}
	if base != nil {
		// The optional base statedb is given, mark the start point as parent block
		statedb, database, report = base, base.Database(), false
		current = k.blockchain.GetBlockByHeight(block.Height() - 1)
	} else {
		// Otherwise try to reexec blocks until we find a state or reach our limit
		current = block

		// Create an ephemeral trie.Database for isolating the live one. Otherwise
		// the internal junks created by tracing will be persisted into the disk.
		database = state.NewDatabase(k.kaiDb.DB())

		// If we didn't check the dirty database, do check the clean one, otherwise
		// we would rewind past a persisted block (specific corner case is chain
		// tracing from the genesis).
		if !checkLive {
			statedb, err = state.New(log.New(), current.AppHash(), database)
			if err == nil {
				return statedb, nil
			}
		}
		// Database does not have the state for the given block, try to regenerate
		for i := uint64(0); i < reexec; i++ {
			if current.Height() == 0 {
				return nil, errors.New("genesis state is missing")
			}
			parent := k.blockchain.GetBlockByHeight(block.Height() - 1)
			if parent == nil {
				return nil, fmt.Errorf("missing block %d", current.Height()-1)
			}
			current = parent

			statedb, err = state.New(log.New(), current.AppHash(), database)
			if err == nil {
				break
			}
		}
		if err != nil {
			switch err.(type) {
			case *trie.MissingNodeError:
				return nil, fmt.Errorf("required historical state unavailable (reexec=%d)", reexec)
			default:
				return nil, err
			}
		}
	}
	// State was available at historical point, regenerate
	var (
		start  = time.Now()
		logged time.Time
		parent common.Hash
	)
	for current.Height() < origin {
		// Print progress logs if long enough time elapsed
		if time.Since(logged) > 8*time.Second && report {
			log.Info("Regenerating historical state", "block", current.Height()+1, "target", origin, "remaining", origin-current.Height()-1, "elapsed", time.Since(start))
			logged = time.Now()
		}
		// Retrieve the next block to regenerate and process it
		next := current.Height() + 1
		if current = k.blockchain.GetBlockByHeight(next); current == nil {
			return nil, fmt.Errorf("block #%d not found", next)
		}
		_, _, _, err := k.blockchain.Processor().Process(current, statedb, kvm.Config{})
		if err != nil {
			return nil, fmt.Errorf("processing block %d failed: %v", current.Height(), err)
		}
		// Finalize the state so any modifications are written to the trie
		root, err := statedb.Commit(true)
		if err != nil {
			return nil, err
		}
		statedb, err = state.New(log.New(), root, database)
		if err != nil {
			return nil, fmt.Errorf("state reset after block %d failed: %v", current.Height(), err)
		}
		database.TrieDB().Reference(root, common.Hash{})
		if parent != (common.Hash{}) {
			database.TrieDB().Dereference(parent)
		}
		parent = root
	}
	if report {
		nodes, imgs := database.TrieDB().Size()
		log.Info("Historical state regenerated", "block", current.Height(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	}
	return statedb, nil
}

// stateAtTransaction returns the execution environment of a certain transaction.
func (k *KardiaService) stateAtTransaction(block *types.Block, txIndex int, reexec uint64) (blockchain.Message, kvm.Context, *state.StateDB, error) {
	// Short circuit if it's genesis block.
	if block.Height() == 0 {
		return nil, kvm.Context{}, nil, errors.New("no transaction in genesis")
	}
	// Create the parent state database
	parent := k.blockchain.GetBlockByHeight(block.Height() - 1)
	if parent == nil {
		return nil, kvm.Context{}, nil, fmt.Errorf("parent %#x not found", block.Height()-1)
	}
	// Lookup the statedb of parent block from the live database,
	// otherwise regenerate it on the flight.
	statedb, err := k.stateAtBlock(parent, reexec, nil, true)
	if err != nil {
		return nil, kvm.Context{}, nil, err
	}
	if txIndex == 0 && len(block.Transactions()) == 0 {
		return nil, kvm.Context{}, statedb, nil
	}
	// Recompute transactions up to the target index.
	signer := types.HomesteadSigner{}
	for idx, tx := range block.Transactions() {
		// Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer)
		txContext := blockchain.NewKVMTxContext(msg)
		context := vm.NewKVMContext(msg, block.Header(), k.blockchain)
		if idx == txIndex {
			return msg, context, statedb, nil
		}
		// Not yet the searched for transaction, execute on top of the current state
		vmenv := kvm.NewKVM(context, txContext, statedb, configs.MainnetChainConfig, kvm.Config{})
		statedb.Prepare(tx.Hash(), block.Hash(), idx)
		if _, err := blockchain.ApplyMessage(vmenv, msg, new(types.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, kvm.Context{}, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		}
		// Ensure any modifications are committed to the state
		// Only delete empty objects if EIP158/161 (a.k.a Spurious Dragon) is in effect
		statedb.Finalise(true)
	}
	return nil, kvm.Context{}, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
}
