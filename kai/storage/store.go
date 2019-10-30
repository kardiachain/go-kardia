package storage

import (
	"errors"
	"fmt"
	"log"

	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
)

type Database interface {
	Put(key []byte, value []byte)
	Get(key []byte) []byte
	Has(key []byte) bool
}

// Store store blockchain data
type Store struct {
	db     Database
	logger log.Logger
}

func NewStore(db Database) *Store {
	return &Store{
		db: db
	}
}

// ReadSeenCommit read seen commit
func (s Store) ReadSeenCommit(height uint64) *types.Commit {
	commit := new(types.Commit)
	commitBytes := s.db.Get(calcSeenCommitKey(height))
	if err := rlp.DecodeBytes(commitBytes, commit); err != nil {
		panic(errors.New("Reading seen commit error"))
	}
	return commit
}

// ReadCommit read commit
func (s Store) ReadCommit(height int64) *types.Commit {
	commit := new(types.Commit)
	commitBytes := s.db.Get(calCommitKey(height))
	if err := rlp.DecodeBytes(commitBytes, commit); err != nil {
		panic(errors.New("Reading commit error"))
	}
	return commit
}

// ReadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (s Store) ReadBlockMeta(height uint64) *types.BlockMeta {
	var blockMeta = new(types.BlockMeta)

	metaBytes := s.db.Get(calcBlockMetaKey(height))
	if err := rlp.DecodeBytes(metaBytes, blockMeta); err != nil {
		panic(errors.New("Reading block meta error"))
	}
	return blockMeta
}

// ReadBlock read block
func (s Store) ReadBlock(height uint64) *types.Block {
	blockMeta := s.ReadBlockMeta(height)
	buf := []byte{}
	for i := 0; i < blockMeta.BlockID.PartHeaders.Total; i++ {
		part := s.ReadBlockPart(height, i)
		buf = append(buf, part.Bytes...)
	}

	block := new(types.Block)
	if err := rlp.DecodeBytes(buf, block); err != nil {
		panic(errors.New("Reading block error"))
	}
	return block
}

// ReadBlockPart read block part
func (s Store) ReadBlockPart(height uint64, index int) *types.Part {
	part := new(types.Part)
	partBytes := s.db.Get(calcBlockMetaKey(height))
	if err := rlp.DecodeBytes(metaBytes, part); err != nil {
		panic(errors.New("Reading block meta error"))
	}
	return part
}

// WriteBlock write block to database
func (s *Store) WriteBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) *types.Block {
	height := block.Height()

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	metaBytes, _ := rlp.EncodeToBytes(blockMeta)
	s.Put(calcBlockMetaKey(height), metaBytes)

	// Save block part
	for i := 0; i < blockParts.Total(); i++ {
		part := blockParts.GetPart(i)
		s.writeBlockPart(height, i, part)
	}

	// Save commint
	lastCommitBytes, _ := rlp.EncodeToBytes(block.LastCommit())
	s.Put(calCommitKey(height), metaBytes)

	// Save seen commint
	seenCommitBytes, _ := rlp.EncodeToBytes(seenCommit)
	s.Put(calcSeenCommitKey(height), seenCommit)
}

func (s Store) writeBlockPart(height uint64, index int, part *types.Part) {
	metaBytes, _ := rlp.EncodeToBytes(part)
	db.Put(calcBlockPartKey(height, index), part)
}

//-----------------------------------------------------------------------------

func calcBlockPartKey(height uint64, partIndex int) []byte {
	return []byte(fmt.Sprintf("p:%v:%v", height, partIndex))
}

func calcBlockMetaKey(height uint64) []byte {
	return []byte(fmt.Sprintf("h:%v", height))
}

func calcSeenCommitKey(height uint64) []byte {
	return []byte(fmt.Sprintf("sc:%v", height))
}

func calCommitKey(height uint64) []byte {
	return []byte(fmt.Sprintf("c:%v", height))
}
