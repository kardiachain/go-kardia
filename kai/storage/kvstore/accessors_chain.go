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

package kvstore

import (
	"encoding/binary"

	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

// ReadTxIndexTail retrieves the number of oldest indexed block
// whose transaction indices has been indexed. If the corresponding entry
// is non-existent in database it means the indexing has been finished.
func ReadTxIndexTail(db kaidb.KeyValueReader) *uint64 {
	data, _ := db.Get(txIndexTailKey)
	if len(data) != 8 {
		return nil
	}
	height := binary.BigEndian.Uint64(data)
	return &height
}

// WriteTxIndexTail stores the number of oldest indexed block
// into database.
func WriteTxIndexTail(db kaidb.KeyValueWriter, height uint64) {
	if err := db.Put(txIndexTailKey, encodeBlockHeight(height)); err != nil {
		log.Crit("Failed to store the transaction index tail", "err", err)
	}
}

// ReadCanonicalBodyRLP retrieves the block body (transactions) for the canonical
// block at height, in RLP encoding.
func ReadCanonicalBodyRLP(db kaidb.Reader, height uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(height, ReadCanonicalHash(db, height)))
	return data
}
