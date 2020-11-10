/*
 *  Copyright 2020 KardiaChain
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
	_default "github.com/kardiachain/go-kardiamain/configs/default"
	kaiproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

const (
	// MaxBlockSizeBytes is the maximum permitted size of the blocks.
	MaxBlockSizeBytes = 104857600 // 100MB

	// BlockPartSizeBytes is the size of one block part.
	BlockPartSizeBytes = 65536 // 64kB

	// MaxBlockPartsCount is the maximum number of block parts.
	MaxBlockPartsCount = (MaxBlockSizeBytes / BlockPartSizeBytes) + 1
)

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *kaiproto.ConsensusParams {
	return _default.ConsensusParams()
}

// DefaultBlockParams returns a default BlockParams.
func DefaultBlockParams() kaiproto.BlockParams {
	return _default.BlockParams()
}

// DefaultEvidenceParams returns a default EvidenceParams.
func DefaultEvidenceParams() kaiproto.EvidenceParams {
	return _default.EvidenceParams()
}

// DefaultValidatorParams returns a default ValidatorParams, which allows
// only ed25519 pubkeys.
func DefaultValidatorParams() kaiproto.ValidatorParams {
	return kaiproto.ValidatorParams{}
}
