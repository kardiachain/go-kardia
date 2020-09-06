package types

import (
	"github.com/kardiachain/go-kardiamain/lib/strings"
)

const (
	// MaxBlockSizeBytes is the maximum permitted size of the blocks.
	MaxBlockSizeBytes = 104857600 // 100MB

	// BlockPartSizeBytes is the size of one block part.
	BlockPartSizeBytes = 65536 // 64kB

	// MaxBlockPartsCount is the maximum number of block parts.
	MaxBlockPartsCount = (MaxBlockSizeBytes / BlockPartSizeBytes) + 1
)

// BlockParams define limits on the block size and gas plus minimum time
// between blocks.
type BlockParams struct {
	MaxBytes uint64 `json:"max_bytes"`
	MaxGas   uint64 `json:"max_gas"`
	// Minimum time increment between consecutive blocks (in milliseconds)
	// Not exposed to the application.
	TimeIotaMs uint64 `json:"time_iota_ms"`
}

// EvidenceParams determine how we handle evidence of malfeasance.
type EvidenceParams struct {
	MaxAgeNumBlocks uint64 `json:"max_age_num_blocks"` // only accept new evidence more recent than this
	MaxAgeDuration  uint   `json:"max_age_duration"`
}

// ConsensusParams contains consensus critical parameters that determine the
// validity of blocks.
type ConsensusParams struct {
	Evidence  EvidenceParams  `json:"evidence"`
	Block     BlockParams     `json:"block"`
	Validator ValidatorParams `json:"validator"`
}

// ValidatorParams restrict the public key types validators can use.
// NOTE: uses pubkey
type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

// Equals ...
func (params *ConsensusParams) Equals(params2 *ConsensusParams) bool {
	return params.Block == params2.Block &&
		params.Evidence == params2.Evidence &&
		strings.StringSliceEqual(params.Validator.PubKeyTypes, params2.Validator.PubKeyTypes)
}
