// Package _default
package _default

import (
	"time"

	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
	kaiproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

const (
	TimeoutPropose              time.Duration = 3000 * time.Millisecond
	TimeoutProposeDelta         time.Duration = 500 * time.Millisecond
	TimeoutPrevote              time.Duration = 1000 * time.Millisecond
	TimeoutPrevoteDelta         time.Duration = 500 * time.Millisecond
	TimeoutPrecommit            time.Duration = 1000 * time.Millisecond
	TimeoutPrecommitDelta       time.Duration = 500 * time.Millisecond
	TimeoutCommit               time.Duration = 1000 * time.Millisecond
	IsSkipTimeoutCommit         bool          = false
	IsCreateEmptyBlocks         bool          = true
	CreateEmptyBlocksInterval   time.Duration = 1 * time.Millisecond
	PeerGossipSleepDuration     time.Duration = 100 * time.Millisecond
	PeerQueryMaj23SleepDuration time.Duration = 2000 * time.Millisecond

	BlockGasLimit uint64 = 200000000 // Gas limit of one block.
	BlockMaxBytes int64  = 104857600 // Block max size bytes: 10mbs
	TimeIotaMs    int64  = 1000

	MaxAgeNumBlocks int64         = 100000
	MaxAgeDuration  time.Duration = 48 * time.Hour
	MaxBytes        int64         = 1048576
)

var (
	consensusConfig = &typesCfg.ConsensusConfig{
		TimeoutPropose:              TimeoutPropose,
		TimeoutProposeDelta:         TimeoutProposeDelta,
		TimeoutPrevote:              TimeoutPrevote,
		TimeoutPrevoteDelta:         TimeoutPrevoteDelta,
		TimeoutPrecommit:            TimeoutPrecommit,
		TimeoutPrecommitDelta:       TimeoutPrecommitDelta,
		TimeoutCommit:               TimeoutCommit,
		IsSkipTimeoutCommit:         IsSkipTimeoutCommit,
		IsCreateEmptyBlocks:         IsCreateEmptyBlocks,
		CreateEmptyBlocksInterval:   CreateEmptyBlocksInterval,
		PeerGossipSleepDuration:     PeerGossipSleepDuration,
		PeerQueryMaj23SleepDuration: PeerQueryMaj23SleepDuration,
	}
	blockParams = kaiproto.BlockParams{
		MaxBytes:   BlockMaxBytes,
		MaxGas:     BlockGasLimit,
		TimeIotaMs: TimeIotaMs,
	}
	evidenceParams = kaiproto.EvidenceParams{
		MaxAgeNumBlocks: MaxAgeNumBlocks, // 27.8 hrs at 1block/s
		MaxAgeDuration:  MaxAgeDuration,
		MaxBytes:        MaxBytes, // 1MB
	}
	consensusParams = &kaiproto.ConsensusParams{
		Block:    blockParams,
		Evidence: evidenceParams,
	}
)

func Consensus() *typesCfg.Consensus {
	return &typesCfg.Consensus{}
}

func ConsensusConfig() *typesCfg.ConsensusConfig {
	return consensusConfig
}

func ConsensusParams() *kaiproto.ConsensusParams {
	return consensusParams
}

func BlockParams() kaiproto.BlockParams {
	return blockParams
}

func EvidenceParams() kaiproto.EvidenceParams {
	return evidenceParams
}
