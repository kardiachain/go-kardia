package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/merkle"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
)

const (
	// MaxEvidenceBytes is a maximum size of any evidence (including amino overhead).
	MaxEvidenceBytes int64 = 436
)

// ErrEvidenceInvalid wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrEvidenceInvalid struct {
	Evidence   Evidence
	ErrorValue error
}

// NewErrEvidenceInvalid returns a new EvidenceInvalid with the given err.
func NewErrEvidenceInvalid(ev Evidence, err error) *ErrEvidenceInvalid {
	return &ErrEvidenceInvalid{ev, err}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceInvalid) Error() string {
	return fmt.Sprintf("Invalid evidence: %v. Evidence: %v", err.ErrorValue, err.Evidence)
}

// ErrEvidenceOverflow is for when there is too much evidence in a block.
type ErrEvidenceOverflow struct {
	MaxNum int64
	GotNum int64
}

// NewErrEvidenceOverflow returns a new ErrEvidenceOverflow where got > max.
func NewErrEvidenceOverflow(max, got int64) *ErrEvidenceOverflow {
	return &ErrEvidenceOverflow{max, got}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceOverflow) Error() string {
	return fmt.Sprintf("Too much evidence: Max %d, got %d", err.MaxNum, err.GotNum)
}

//-------------------------------------------

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() *common.BigInt                              // height of the equivocation
	Address() common.Address                             // address of the equivocating validator
	Bytes() []byte                                       // bytes which compromise the evidence
	Hash() common.Hash                                   // hash of the evidence
	Verify(chainID string, pubKey ecdsa.PublicKey) error // verify the evidence
	Equal(Evidence) bool                                 // check equality of evidence

	ValidateBasic() error
	String() string
}

const (
	MaxEvidenceBytesDenominator = 10
)

// MaxEvidencePerBlock returns the maximum number of evidences
// allowed in the block and their maximum total size (limitted to 1/10th
// of the maximum block size).
// TODO: change to a constant, or to a fraction of the validator set size.
// See https://github.com/tendermint/tendermint/issues/2590
func MaxEvidencePerBlock(blockMaxBytes int64) (int64, int64) {
	maxBytes := blockMaxBytes / MaxEvidenceBytesDenominator
	maxNum := maxBytes / MaxEvidenceBytes
	return maxNum, maxBytes
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	PubKey ecdsa.PublicKey
	VoteA  *Vote
	VoteB  *Vote
}

var _ Evidence = &DuplicateVoteEvidence{}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(pubkey ecdsa.PublicKey, vote1 *Vote, vote2 *Vote) *DuplicateVoteEvidence {
	var voteA, voteB *Vote
	if vote1 == nil || vote2 == nil {
		return nil
	}
	if strings.Compare(vote1.BlockID.Key(), vote2.BlockID.Key()) == -1 {
		voteA = vote1
		voteB = vote2
	} else {
		voteA = vote2
		voteB = vote1
	}
	return &DuplicateVoteEvidence{
		PubKey: pubkey,
		VoteA:  voteA,
		VoteB:  voteB,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("VoteA: %v; VoteB: %v", dve.VoteA, dve.VoteB)

}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() *common.BigInt {
	return dve.VoteA.Height
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Address() common.Address {
	return crypto.PubkeyToAddress(dve.PubKey)
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Bytes() []byte {
	return dve.Hash().Bytes()
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() common.Hash {
	return rlpHash(dve)
}

// Verify returns an error if the two votes aren't conflicting.
// To be conflicting, they must be from the same validator, for the same H/R/S, but for different blocks.
func (dve *DuplicateVoteEvidence) Verify(chainID string, pubKey ecdsa.PublicKey) error {
	// H/R/S must be the same
	if !dve.VoteA.Height.Equals(dve.VoteB.Height) ||
		!dve.VoteA.Round.Equals(dve.VoteB.Round) ||
		dve.VoteA.Type != dve.VoteB.Type {
		return fmt.Errorf("duplicateVoteEvidence Error: H/R/S does not match. Got %v and %v", dve.VoteA, dve.VoteB)
	}

	// Address must be the same
	if !dve.VoteA.ValidatorAddress.Equal(dve.VoteB.ValidatorAddress) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator addresses do not match. Got %X and %X",
			dve.VoteA.ValidatorAddress,
			dve.VoteB.ValidatorAddress,
		)
	}

	// Index must be the same
	if !dve.VoteA.ValidatorIndex.Equals(dve.VoteB.ValidatorIndex) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator indices do not match. Got %d and %d",
			dve.VoteA.ValidatorIndex.Int32(),
			dve.VoteB.ValidatorIndex.Int32(),
		)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equal(dve.VoteB.BlockID) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: BlockIDs are the same (%v) - not a real duplicate vote",
			dve.VoteA.BlockID,
		)
	}

	// pubkey must match address (this should already be true, sanity check)
	addr := dve.VoteA.ValidatorAddress
	if !crypto.PubkeyToAddress(pubKey).Equal(addr) {
		return fmt.Errorf("duplicateVoteEvidence FAILED SANITY CHECK - address (%X) doesn't match pubkey (%v - %X)",
			addr, pubKey, crypto.PubkeyToAddress(pubKey))
	}
	// Signatures must be valid
	if !crypto.VerifySignature(pubKey, rlpHash(dve.VoteA.SignBytes(chainID)).Bytes(), dve.VoteA.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteA: %v", ErrVoteInvalidSignature)
	}
	if !crypto.VerifySignature(pubKey, rlpHash(dve.VoteB.SignBytes(chainID)).Bytes(), dve.VoteB.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteB: %v", ErrVoteInvalidSignature)
	}

	return nil
}

// Equal checks if two pieces of evidence are equal.
func (dve *DuplicateVoteEvidence) Equal(ev Evidence) bool {
	if _, ok := ev.(*DuplicateVoteEvidence); !ok {
		return false
	}
	return dve.Hash().Equal(ev.Hash())
}

// ValidateBasic performs basic validation.
func (dve *DuplicateVoteEvidence) ValidateBasic() error {
	if len(dve.Address().Bytes()) == 0 {
		return errors.New("empty PubKey")
	}
	if dve.VoteA == nil || dve.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	}

	if err := dve.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %v", err)
	}
	if err := dve.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %v", err)
	}

	// Enforce Votes are lexicographically sorted on blockID
	if strings.Compare(dve.VoteA.BlockID.Key(), dve.VoteB.BlockID.Key()) >= 0 {
		return errors.New("duplicate votes in invalid order")
	}

	return nil
}

func (dve *DuplicateVoteEvidence) DecodeRLP(s *rlp.Stream) error {
	// Retrieve the entire receipt blob as we need to try multiple decoders
	blob, err := s.Raw()
	if err != nil {
		return err
	}

	var stored duplicateVoteEvidenceRLP
	if err := rlp.DecodeBytes(blob, &stored); err != nil {
		return err
	}

	dve.VoteA = stored.VoteA
	dve.VoteB = stored.VoteB

	return nil
}

func (dve *DuplicateVoteEvidence) EncodeRLP(w io.Writer) error {
	enc := &duplicateVoteEvidenceRLP{
		Address: dve.Address(),
		VoteA:   dve.VoteA,
		VoteB:   dve.VoteB,
	}

	return rlp.Encode(w, enc)
}

type duplicateVoteEvidenceRLP struct {
	Address common.Address
	VoteA   *Vote
	VoteB   *Vote
}

//-----------------------------------------------------------------

// UNSTABLE
type MockRandomGoodEvidence struct {
	MockGoodEvidence
	randBytes []byte
}

var _ Evidence = &MockRandomGoodEvidence{}

// UNSTABLE
func NewMockRandomGoodEvidence(height *common.BigInt, address common.Address, randBytes []byte) MockRandomGoodEvidence {
	return MockRandomGoodEvidence{
		MockGoodEvidence{height, address}, randBytes,
	}
}

func (e MockRandomGoodEvidence) Hash() common.Hash {
	return common.BytesToHash([]byte(fmt.Sprintf("%d-%x", e.Height_.Uint64(), e.randBytes)))
}

// UNSTABLE
type MockGoodEvidence struct {
	Height_  *common.BigInt
	Address_ common.Address
}

var _ Evidence = &MockGoodEvidence{}

// UNSTABLE
func NewMockGoodEvidence(height *common.BigInt, idx int, address common.Address) MockGoodEvidence {
	return MockGoodEvidence{height, address}
}

func (e MockGoodEvidence) Height() *common.BigInt  { return e.Height_ }
func (e MockGoodEvidence) Address() common.Address { return e.Address_ }
func (e MockGoodEvidence) Hash() common.Hash {
	return common.BytesToHash([]byte(fmt.Sprintf("%d-%x", e.Height_.Int32(), e.Address_)))
}
func (e MockGoodEvidence) Bytes() []byte {
	return []byte(fmt.Sprintf("%d-%x", e.Height_.Int64(), e.Address_))
}
func (e MockGoodEvidence) Verify(chainID string, pubKey ecdsa.PublicKey) error { return nil }
func (e MockGoodEvidence) Equal(ev Evidence) bool {
	e2 := ev.(MockGoodEvidence)
	return e.Height_ == e2.Height_ && e.Address_.Equal(e2.Address_)
}
func (e MockGoodEvidence) ValidateBasic() error { return nil }
func (e MockGoodEvidence) String() string {
	return fmt.Sprintf("GoodEvidence: %d/%s", e.Height_.Int64(), e.Address_)
}

// UNSTABLE
type MockBadEvidence struct {
	MockGoodEvidence
}

func (e MockBadEvidence) Verify(chainID string, pubKey ecdsa.PublicKey) error {
	return fmt.Errorf("mockBadEvidence")
}
func (e MockBadEvidence) Equal(ev Evidence) bool {
	e2 := ev.(MockBadEvidence)
	return e.Height_ == e2.Height_ &&
		e.Address_.Equal(e2.Address_)
}
func (e MockBadEvidence) ValidateBasic() error { return nil }
func (e MockBadEvidence) String() string {
	return fmt.Sprintf("BadEvidence: %d/%s", e.Height_.Int64(), e.Address_)
}

//-------------------------------------------

// EvidenceList is a list of Evidence. Evidences is not a word.
type EvidenceList []Evidence

// Hash returns the simple merkle root hash of the EvidenceList.
func (evl EvidenceList) Hash() []byte {
	// These allocations are required because Evidence is not of type Bytes, and
	// golang slices can't be typed cast. This shouldn't be a performance problem since
	// the Evidence size is capped.
	evidenceBzs := make([][]byte, len(evl))
	for i := 0; i < len(evl); i++ {
		evidenceBzs[i] = evl[i].Bytes()
	}
	return merkle.SimpleHashFromByteSlices(evidenceBzs)
}

func (evl EvidenceList) String() string {
	s := ""
	for _, e := range evl {
		s += fmt.Sprintf("%s\t\t", e)
	}
	return s
}

// Has returns true if the evidence is in the EvidenceList.
func (evl EvidenceList) Has(evidence Evidence) bool {
	for _, ev := range evl {
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
}
