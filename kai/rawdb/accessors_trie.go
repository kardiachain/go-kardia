// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>

package rawdb

import (
	"sync"

	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"golang.org/x/crypto/sha3"
)

// HashScheme is the legacy hash-based state scheme with which trie nodes are
// stored in the disk with node hash as the database key. The advantage of this
// scheme is that different versions of trie nodes can be stored in disk, which
// is very beneficial for constructing archive nodes. The drawback is it will
// store different trie nodes on the same path to different locations on the disk
// with no data locality, and it's unfriendly for designing state pruning.
//
// Now this scheme is still kept for backward compatibility, and it will be used
// for archive node and some other tries(e.g. light trie).
const HashScheme = "hashScheme"

// PathScheme is the new path-based state scheme with which trie nodes are stored
// in the disk with node path as the database key. This scheme will only store one
// version of state data in the disk, which means that the state pruning operation
// is native. At the same time, this scheme will put adjacent trie nodes in the same
// area of the disk with good data locality property. But this scheme needs to rely
// on extra state diffs to survive deep reorg.
const PathScheme = "pathScheme"

// nodeHasher used to derive the hash of trie node.
type nodeHasher struct{ sha crypto.KeccakState }

var hasherPool = sync.Pool{
	New: func() interface{} { return &nodeHasher{sha: sha3.NewLegacyKeccak256().(crypto.KeccakState)} },
}

func newNodeHasher() *nodeHasher       { return hasherPool.Get().(*nodeHasher) }
func returnHasherToPool(h *nodeHasher) { hasherPool.Put(h) }

func (h *nodeHasher) hashData(data []byte) (n common.Hash) {
	h.sha.Reset()
	h.sha.Write(data)
	h.sha.Read(n[:])
	return n
}

// WriteLegacyTrieNode writes the provided legacy trie node to database.
func WriteLegacyTrieNode(db kaidb.KeyValueWriter, hash common.Hash, node []byte) {
	if err := db.Put(hash.Bytes(), node); err != nil {
		log.Crit("Failed to store legacy trie node", "err", err)
	}
}

// ReadLegacyTrieNode retrieves the legacy trie node with the given
// associated node hash.
func ReadLegacyTrieNode(db kaidb.KeyValueReader, hash common.Hash) []byte {
	data, err := db.Get(hash.Bytes())
	if err != nil {
		return nil
	}
	return data
}

// HasLegacyTrieNode checks if the trie node with the provided hash is present in db.
func HasLegacyTrieNode(db kaidb.KeyValueReader, hash common.Hash) bool {
	ok, _ := db.Has(hash.Bytes())
	return ok
}