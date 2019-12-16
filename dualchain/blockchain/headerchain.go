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

package blockchain

import (
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

const (
	headerCacheLimit = 512
	heightCacheLimit = 2048
)

// Header of dual's blockchain.
type DualHeaderChain struct {
	config *types.ChainConfig

	kaiDb types.StoreDB

	genesisHeader *types.Header

	currentHeader     atomic.Value // Current head of the header chain (may be above the block chain!)
	currentHeaderHash common.Hash  // Hash of the current head of the header chain (prevent recomputing all the time)

	headerCache *lru.Cache // Cache for the most recent block headers
	heightCache *lru.Cache // Cache for the most recent block height
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the DualHeaderChain's internal cache.
func (dhc *DualHeaderChain) CurrentHeader() *types.Header {
	return dhc.currentHeader.Load().(*types.Header)
}

// NewHeaderChain creates a new DualHeaderChain structure.
//  getValidator should return the parent's validator
//  procInterrupt points to the parent's interrupt semaphore
//  wg points to the parent's shutdown wait group
func NewHeaderChain(kaiDb types.StoreDB, config *types.ChainConfig) (*DualHeaderChain, error) {
	headerCache, _ := lru.New(headerCacheLimit)
	heightCache, _ := lru.New(heightCacheLimit)

	dhc := &DualHeaderChain{
		config:      config,
		kaiDb:       kaiDb,
		headerCache: headerCache,
		heightCache: heightCache,
	}

	dhc.genesisHeader = dhc.GetHeaderByHeight(0)
	if dhc.genesisHeader == nil {
		return nil, ErrNoGenesis
	}

	dhc.currentHeader.Store(dhc.genesisHeader)
	if head := kaiDb.ReadHeadBlockHash(); head != (common.Hash{}) {
		if chead := dhc.GetHeaderByHash(head); chead != nil {
			dhc.currentHeader.Store(chead)
		}
	}
	dhc.currentHeaderHash = dhc.CurrentHeader().Hash()

	return dhc, nil
}

// GetHeaderByheight retrieves a block header from the database by height,
// caching it (associated with its hash) if found.
func (dhc *DualHeaderChain) GetHeaderByHeight(height uint64) *types.Header {
	hash := dhc.kaiDb.ReadCanonicalHash(height)
	if hash == (common.Hash{}) {
		return nil
	}
	return dhc.GetHeader(hash, height)
}

// GetHeader retrieves a block header from the database by hash and height,
// caching it if found.
func (dhc *DualHeaderChain) GetHeader(hash common.Hash, height uint64) *types.Header {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := dhc.headerCache.Get(hash); ok {
		return header.(*types.Header)
	}
	header := dhc.kaiDb.ReadHeader(hash, height)
	if header == nil {
		return nil
	}
	// Cache the found header for next time and return
	dhc.headerCache.Add(hash, header)
	return header
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (dhc *DualHeaderChain) GetHeaderByHash(hash common.Hash) *types.Header {
	height := dhc.GetBlockHeight(hash)
	if height == nil {
		return nil
	}
	return dhc.GetHeader(hash, *height)
}

// GetBlockHeight retrieves the block height belonging to the given hash
// from the cache or database
func (dhc *DualHeaderChain) GetBlockHeight(hash common.Hash) *uint64 {
	if cached, ok := dhc.heightCache.Get(hash); ok {
		height := cached.(uint64)
		return &height
	}
	height := dhc.kaiDb.ReadHeaderHeight(hash)
	if height != nil {
		dhc.heightCache.Add(hash, *height)
	}
	return height
}

// SetCurrentHeader sets the current head header of the canonical chain.
func (dhc *DualHeaderChain) SetCurrentHeader(head *types.Header) {
	dhc.kaiDb.WriteHeadHeaderHash(head.Hash())

	dhc.currentHeader.Store(head)
	dhc.currentHeaderHash = head.Hash()
}

// SetGenesis sets a new genesis block header for the chain
func (dhc *DualHeaderChain) SetGenesis(head *types.Header) {
	dhc.genesisHeader = head
}

// DeleteCallback is a callback function that is called by SetHead before
// each header is deleted.
type DeleteCallback func(types.StoreDB, common.Hash, uint64)

// SetHead rewinds the local chain to a new head. Everything above the new head
// will be deleted and the new one set.
func (dhc *DualHeaderChain) SetHead(head uint64, delFn DeleteCallback) {
	height := uint64(0)

	if hdr := dhc.CurrentHeader(); hdr != nil {
		height = hdr.Height
	}

	for hdr := dhc.CurrentHeader(); hdr != nil && hdr.Height > head; hdr = dhc.CurrentHeader() {
		hash := hdr.Hash()
		height := hdr.Height
		if delFn != nil {
			delFn(dhc.kaiDb, hash, height)
		}
		dhc.kaiDb.DeleteBlockMeta(hash, height)

		dhc.currentHeader.Store(dhc.GetHeader(hdr.LastCommitHash, hdr.Height-1))
	}
	// Roll back the canonical chain numbering
	for i := height; i > head; i-- {
		dhc.kaiDb.DeleteCanonicalHash(i)
	}

	// Clear out any stale content from the caches
	dhc.headerCache.Purge()
	dhc.heightCache.Purge()

	if dhc.CurrentHeader() == nil {
		dhc.currentHeader.Store(dhc.genesisHeader)
	}
	dhc.currentHeaderHash = dhc.CurrentHeader().Hash()

	dhc.kaiDb.WriteHeadHeaderHash(dhc.currentHeaderHash)
}
