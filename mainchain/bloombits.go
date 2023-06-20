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
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/kai/rawdb"

	"github.com/kardiachain/go-kardia/kai/kaidb"

	"github.com/kardiachain/go-kardia/lib/log"

	"github.com/kardiachain/go-kardia/lib/bloombits"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

const (
	// bloomServiceThreads is the number of goroutines used globally by an Ethereum
	// instance to service bloombits lookups for all running filters.
	bloomServiceThreads = 16

	// bloomFilterThreads is the number of goroutines used locally per filter to
	// multiplex requests onto the global servicing goroutines.
	bloomFilterThreads = 3

	// bloomRetrievalBatch is the maximum number of bloom bit retrievals to service
	// in a single batch.
	bloomRetrievalBatch = 16

	// bloomRetrievalWait is the maximum time to wait for enough bloom bit requests
	// to accumulate request an entire batch (avoiding hysteresis).
	bloomRetrievalWait = time.Duration(0)

	// bloomThrottling is the time to wait between processing two consecutive index
	// sections. It's useful during chain upgrades to prevent disk overload.
	bloomThrottling = 100 * time.Millisecond

	bloomLogServiceName = "bloombits"
)

// startBloomHandlers starts a batch of goroutines to accept bloom bit database
// retrievals from possibly a range of filters and serving the data to satisfy.
func (k *KardiaService) startBloomHandlers(sectionSize uint64) {
	for i := 0; i < bloomServiceThreads; i++ {
		go func() {
			for {
				select {
				case <-k.closeBloomHandler:
					return

				case request := <-k.bloomRequests:
					task := <-request
					task.Bitsets = make([][]byte, len(task.Sections))
					db := k.blockchain.DB()
					for i, section := range task.Sections {
						head := db.ReadCanonicalHash((section+1)*sectionSize - 1)
						if compVector, err := rawdb.ReadBloomBits(db.DB(), task.Bit, section, head); err == nil {
							if blob, err := common.DecompressBytes(compVector, int(sectionSize/8)); err == nil {
								task.Bitsets[i] = blob
							} else {
								task.Error = err
							}
						} else {
							task.Error = err
						}
					}
					request <- task
				}
			}
		}()
	}
}

// BloomIndexer implements a core.ChainIndexer, building up a rotated bloom bits index
// for the Ethereum header bloom filters, permitting blazing fast filtering.
type BloomIndexer struct {
	sectionSize uint64 // section size to generate bloombits for
	confirmsReq uint64 // Number of confirmations before processing a completed segment

	db      kaidb.Database // database instance to write index data and metadata into
	indexDb kaidb.Database // Prefixed table-view of the db to write index metadata into

	storedSections     uint64 // Number of sections successfully indexed into the database
	checkpointSections uint64 // Number of sections covered by the checkpoint

	ctx       context.Context
	ctxCancel func()

	gen        *bloombits.Generator // generator to rotate the bloom bits crating the bloom index
	section    uint64               // Section is the section number being processed currently
	head       common.Hash          // Head is the hash of the last header processed
	throttling time.Duration        // Disk throttling to prevent a heavy upgrade from hogging resources

	lock sync.Mutex
	log  log.Logger
}

// NewBloomIndexer returns a chain indexer that generates bloom bits data for the
// canonical chain for fast logs filtering.
func NewBloomIndexer(db kaidb.Database, size, confirms uint64) *BloomIndexer {
	backend := &BloomIndexer{
		db:      db,
		indexDb: rawdb.NewMemoryDatabase().DB(),

		sectionSize: size,
		confirmsReq: confirms,
		throttling:  bloomThrottling,
		log:         log.New("type", bloomLogServiceName),
	}
	backend.ctx, backend.ctxCancel = context.WithCancel(context.Background())
	return backend
}

// Reset implements core.ChainIndexerBackend, starting a new bloombits index
// section.
func (b *BloomIndexer) Reset(ctx context.Context, section uint64, lastSectionHead common.Hash) error {
	gen, err := bloombits.NewGenerator(uint(b.sectionSize))
	b.gen, b.section, b.head = gen, section, common.Hash{}
	return err
}

// Process implements core.ChainIndexerBackend, adding a new header's bloom into
// the index.
func (b *BloomIndexer) Process(ctx context.Context, header *types.Header, blockInfo *types.BlockInfo) error {
	b.gen.AddBloom(uint(header.Height-b.section*b.sectionSize), blockInfo.Bloom)
	b.head = header.Hash()
	return nil
}

// Commit implements core.ChainIndexerBackend, finalizing the bloom section and
// writing it out into the database.
func (b *BloomIndexer) Commit() error {
	batch := b.db.NewBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		bits, err := b.gen.Bitset(uint(i))
		if err != nil {
			return err
		}
		rawdb.WriteBloomBits(batch, uint(i), b.section, b.head, common.CompressBytes(bits))
	}
	return batch.Write()
}

// Prune returns an empty error since we don't support pruning here.
func (b *BloomIndexer) Prune(threshold uint64) error {
	return nil
}

// processSection processes an entire section by calling backend functions while
// ensuring the continuity of the passed headers. Since the chain mutex is not
// held while processing, the continuity can be broken by a long reorg, in which
// case the function returns with an error.
func (b *BloomIndexer) processSection(section uint64, lastHead common.Hash) (common.Hash, error) {
	b.log.Trace("Processing new chain section", "section", section)

	// Reset and partial processing
	if err := b.Reset(b.ctx, section, lastHead); err != nil {
		b.setValidSections(0)
		return common.Hash{}, err
	}
	db := b.db.(types.StoreDB)
	for number := section * b.sectionSize; number < (section+1)*b.sectionSize; number++ {
		hash := db.ReadCanonicalHash(number)
		if hash == (common.Hash{}) {
			return common.Hash{}, fmt.Errorf("canonical block #%d unknown", number)
		}
		header := db.ReadHeader(number)
		if header == nil {
			return common.Hash{}, fmt.Errorf("block #%d [%x…] not found", number, hash[:4])
		}
		blockInfo := db.ReadBlockInfo(hash, number, nil)
		if blockInfo == nil {
			return common.Hash{}, fmt.Errorf("block info #%d [%x…] not found", number, hash[:4])
		}
		if err := b.Process(b.ctx, header, blockInfo); err != nil {
			return common.Hash{}, err
		}
		lastHead = header.Hash()
	}
	if err := b.Commit(); err != nil {
		return common.Hash{}, err
	}
	return lastHead, nil
}

// verifyLastHead compares last stored section head with the corresponding block hash in the
// actual canonical chain and rolls back reorged sections if necessary to ensure that stored
// sections are all valid
func (b *BloomIndexer) verifyLastHead() {
	for b.storedSections > 0 && b.storedSections > b.checkpointSections {
		if b.SectionHead(b.storedSections-1) == b.db.(types.StoreDB).ReadCanonicalHash(b.storedSections*b.sectionSize-1) {
			return
		}
		b.setValidSections(b.storedSections - 1)
	}
}

// Sections returns the number of processed sections maintained by the indexer
// and also the information about the last header indexed for potential canonical
// verifications.
func (b *BloomIndexer) Sections() (uint64, uint64, common.Hash) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.verifyLastHead()
	return b.storedSections, b.storedSections*b.sectionSize - 1, b.SectionHead(b.storedSections - 1)
}

// loadValidSections reads the number of valid sections from the index database
// and caches is into the local state.
func (b *BloomIndexer) loadValidSections() {
	data, _ := b.indexDb.Get([]byte("count"))
	if len(data) == 8 {
		b.storedSections = binary.BigEndian.Uint64(data)
	}
}

// setValidSections writes the number of valid sections to the index database
func (b *BloomIndexer) setValidSections(sections uint64) {
	// Set the current number of valid sections in the database
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], sections)
	b.indexDb.Put([]byte("count"), data[:])

	// Remove any reorged sections, caching the valids in the mean time
	for b.storedSections > sections {
		b.storedSections--
		b.removeSectionHead(b.storedSections)
	}
	b.storedSections = sections // needed if new > old
}

// SectionHead retrieves the last block hash of a processed section from the
// index database.
func (b *BloomIndexer) SectionHead(section uint64) common.Hash {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], section)

	hash, _ := b.indexDb.Get(append([]byte("shead"), data[:]...))
	if len(hash) == len(common.Hash{}) {
		return common.BytesToHash(hash)
	}
	return common.Hash{}
}

// setSectionHead writes the last block hash of a processed section to the index
// database.
func (b *BloomIndexer) setSectionHead(section uint64, hash common.Hash) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], section)

	b.indexDb.Put(append([]byte("shead"), data[:]...), hash.Bytes())
}

// removeSectionHead removes the reference to a processed section from the index
// database.
func (b *BloomIndexer) removeSectionHead(section uint64) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], section)

	b.indexDb.Delete(append([]byte("shead"), data[:]...))
}
