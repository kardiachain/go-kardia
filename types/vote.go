package types

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

var (
	ErrVoteUnexpectedStep            = errors.New("Unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("Invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("Invalid validator address")
	ErrVoteInvalidSignature          = errors.New("Invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("Invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("Non-deterministic signature")
	ErrVoteNil                       = errors.New("Nil vote")
)

type ErrVoteConflictingVotes struct {
	*DuplicateVoteEvidence
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("Conflicting votes from validator %v", crypto.PubkeyToAddress(err.PubKey))
}

func NewConflictingVoteError(val *Validator, voteA, voteB *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		&DuplicateVoteEvidence{
			PubKey: val.PubKey,
			VoteA:  voteA,
			VoteB:  voteB,
		},
	}
}

// Types of votes
// TODO Make a new type "VoteType"
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
)

func IsVoteTypeValid(type_ byte) bool {
	switch type_ {
	case VoteTypePrevote:
		return true
	case VoteTypePrecommit:
		return true
	default:
		return false
	}
}

// Represents a prevote, precommit, or commit vote from validators for consensus.
type Vote struct {
	ValidatorAddress cmn.Address `json:"validator_address"`
	ValidatorIndex   *cmn.BigInt `json:"validator_index"`
	Height           *cmn.BigInt `json:"height"`
	Round            *cmn.BigInt `json:"round"`
	Timestamp        *big.Int    `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
	Type             byte        `json:"type"`
	BlockID          BlockID     `json:"block_id"` // zero if vote is nil.
	Signature        []byte      `json:"signature"`
}

func CreateEmptyVote() *Vote {
	return &Vote{
		ValidatorIndex: cmn.NewBigInt(-1),
		Height:         cmn.NewBigInt(-1),
		Round:          cmn.NewBigInt(-1),
		Timestamp:      big.NewInt(0),
	}
}

func (vote *Vote) IsEmpty() bool {
	return vote.ValidatorIndex.EqualsInt(-1) && vote.Height.EqualsInt(-1) && vote.Height.EqualsInt(-1) && vote.Timestamp.Int64() == 0
}

func (vote *Vote) SignBytes(chainID string) []byte {
	bz, err := rlp.EncodeToBytes(CreateCanonicalVote(chainID, vote))
	if err != nil {
		panic(err)
	}
	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

func (vote *Vote) String() string {
	if vote == nil {
		return "nil-Vote"
	}
	if vote.IsEmpty() {
		return "empty-Vote"
	}
	var typeString string
	switch vote.Type {
	case VoteTypePrevote:
		typeString = "Prevote"
	case VoteTypePrecommit:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%v/%v(%v) %X , %v @ %v}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress[:]),
		vote.Height, vote.Round, vote.Type, typeString,
		cmn.Fingerprint(vote.BlockID[:]), vote.Signature,
		time.Unix(vote.Timestamp.Int64(), 0))
}

// UNSTABLE
// XXX: duplicate of p2p.ID to avoid dependence between packages.
// Perhaps we can have a minimal types package containing this (and other things?)
// that both `types` and `p2p` import ?
type P2PID string
