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
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/kardiachain/go-kardiamain/lib/common"
	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/merkle"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

var (
	ErrNilPart          = errors.New("nil Part")
	ErrNilPartSetHeader = errors.New("nil PartSetHeader")
)

type Part struct {
	Index uint32             `json:"index"`
	Bytes []byte             `json:"bytes"`
	Proof merkle.SimpleProof `json:"proof"`
}

// ValidateBasic performs basic validation.
func (part *Part) ValidateBasic() error {
	if part.Index < 0 {
		return errors.New("Negative Index")
	}
	if len(part.Bytes) > BlockPartSizeBytes {
		return fmt.Errorf("Too big (max: %d)", BlockPartSizeBytes)
	}
	return nil
}

func (part *Part) String() string {
	return part.StringIndented("")
}

func (part *Part) StringIndented(indent string) string {
	return fmt.Sprintf(`Part{#%v
%s  Bytes: %X...
%s  Proof: %v
%s}`,
		part.Index,
		indent, cmn.Fingerprint(part.Bytes),
		indent, part.Proof.StringIndented(indent+"  "),
		indent)
}

func (part *Part) ToProto() (*kproto.Part, error) {
	if part == nil {
		return nil, ErrNilPart
	}
	pb := new(kproto.Part)
	proof := part.Proof.ToProto()

	pb.Index = part.Index
	pb.Bytes = part.Bytes
	pb.Proof = *proof

	return pb, nil
}

func PartFromProto(pb *kproto.Part) (*Part, error) {
	if pb == nil {
		return nil, ErrNilPart
	}

	part := new(Part)
	proof, err := merkle.ProofFromProto(&pb.Proof)
	if err != nil {
		return nil, err
	}
	part.Index = pb.Index
	part.Bytes = pb.Bytes
	part.Proof = *proof

	return part, part.ValidateBasic()
}

type PartSetHeader struct {
	Total uint32   `json:"total"`
	Hash  cmn.Hash `json:"hash"`
}

func (psh PartSetHeader) String() string {
	return fmt.Sprintf("%v:%X", psh.Total, psh.Hash.Fingerprint())
}

func (psh PartSetHeader) IsZero() bool {
	return psh.Total == 0 && psh.Hash.IsZero()
}

func (psh PartSetHeader) Equals(other PartSetHeader) bool {
	return psh.Total == other.Total && common.Hash.Equal(psh.Hash, other.Hash)
}

// ValidateBasic performs basic validation.
func (psh PartSetHeader) ValidateBasic() error {
	// Hash can be empty in case of POLBlockID.PartSetHeader in Proposal.
	if err := ValidateHash(psh.Hash); err != nil {
		return fmt.Errorf("wrong Hash: %w", err)
	}
	return nil
}

// ToProto converts PartSetHeader to protobuf
func (psh *PartSetHeader) ToProto() kproto.PartSetHeader {
	if psh == nil {
		return kproto.PartSetHeader{}
	}

	return kproto.PartSetHeader{
		Total: psh.Total,
		Hash:  psh.Hash.Bytes(),
	}
}

// FromProto sets a protobuf PartSetHeader to the given pointer
func PartSetHeaderFromProto(ppsh *kproto.PartSetHeader) (*PartSetHeader, error) {
	if ppsh == nil {
		return nil, ErrNilPartSetHeader
	}
	psh := new(PartSetHeader)
	psh.Total = ppsh.Total
	psh.Hash = common.BytesToHash(ppsh.Hash)

	return psh, psh.ValidateBasic()
}

type PartSet struct {
	total uint32
	hash  common.Hash

	mtx           sync.Mutex
	parts         []*Part
	partsBitArray *cmn.BitArray
	count         uint32
}

// Returns an immutable, full PartSet from the data bytes.
// The data bytes are split into "partSize" chunks, and merkle tree computed.
func NewPartSetFromData(data []byte, partSize uint32) *PartSet {
	// divide data into 4kb parts.
	total := (uint32(len(data)) + partSize - 1) / partSize
	parts := make([]*Part, total)
	partsBytes := make([][]byte, total)
	partsBitArray := cmn.NewBitArray(int(total))
	for i := uint32(0); i < total; i++ {
		part := &Part{
			Index: i,
			Bytes: data[i*partSize : cmn.MinInt(len(data), int((i+1)*partSize))],
		}
		parts[i] = part
		partsBytes[i] = part.Bytes
		partsBitArray.SetIndex(int(i), true)
	}
	// Compute merkle proofs
	root, proofs := merkle.SimpleProofsFromByteSlices(partsBytes)
	for i := uint32(0); i < total; i++ {
		parts[i].Proof = *proofs[i]
	}
	return &PartSet{
		total:         total,
		hash:          common.BytesToHash(root),
		parts:         parts,
		partsBitArray: partsBitArray,
		count:         total,
	}
}

// Returns an empty PartSet ready to be populated.
func NewPartSetFromHeader(header PartSetHeader) *PartSet {
	return &PartSet{
		total:         header.Total,
		hash:          header.Hash,
		parts:         make([]*Part, header.Total),
		partsBitArray: cmn.NewBitArray(int(header.Total)),
		count:         0,
	}
}

func (ps *PartSet) Header() PartSetHeader {
	if ps == nil {
		return PartSetHeader{}
	}
	return PartSetHeader{
		Total: ps.total,
		Hash:  ps.hash,
	}
}

func (ps *PartSet) HasHeader(header PartSetHeader) bool {
	if ps == nil {
		return false
	}
	return ps.Header().Equals(header)
}

func (ps *PartSet) BitArray() *cmn.BitArray {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.partsBitArray.Copy()
}

func (ps *PartSet) Hash() common.Hash {
	if ps == nil {
		return common.Hash{}
	}
	return ps.hash
}

func (ps *PartSet) HashesTo(hash common.Hash) bool {
	if ps == nil {
		return false
	}
	return ps.Hash().Equal(hash)
}

func (ps *PartSet) Count() uint32 {
	if ps == nil {
		return 0
	}
	return ps.count
}

func (ps *PartSet) Total() uint32 {
	if ps == nil {
		return 0
	}
	return ps.total
}

func (ps *PartSet) AddPart(part *Part) (bool, error) {
	if ps == nil {
		return false, nil
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Invalid part index
	if part.Index >= ps.total {
		return false, ErrPartSetUnexpectedIndex
	}

	// If part already exists, return false.
	if ps.parts[part.Index] != nil {
		return false, nil
	}

	// Check hash proof
	if part.Proof.Verify(ps.Hash().Bytes(), part.Bytes) != nil {
		return false, ErrPartSetInvalidProof
	}

	// Add part
	ps.parts[part.Index] = part
	ps.partsBitArray.SetIndex(int(part.Index), true)
	ps.count++
	return true, nil
}

func (ps *PartSet) GetPart(index int) *Part {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.parts[index]
}

func (ps *PartSet) IsComplete() bool {
	return ps.count == ps.total
}

func (ps *PartSet) GetReader() io.Reader {
	if !ps.IsComplete() {
		cmn.PanicSanity("Cannot GetReader() on incomplete PartSet")
	}
	return NewPartSetReader(ps.parts)
}

type PartSetReader struct {
	i      int
	parts  []*Part
	reader *bytes.Reader
}

func NewPartSetReader(parts []*Part) *PartSetReader {
	return &PartSetReader{
		i:      0,
		parts:  parts,
		reader: bytes.NewReader(parts[0].Bytes),
	}
}

func (psr *PartSetReader) Read(p []byte) (n int, err error) {
	readerLen := psr.reader.Len()
	if readerLen >= len(p) {
		return psr.reader.Read(p)
	} else if readerLen > 0 {
		n1, err := psr.Read(p[:readerLen])
		if err != nil {
			return n1, err
		}
		n2, err := psr.Read(p[readerLen:])
		return n1 + n2, err
	}

	psr.i++
	if psr.i >= len(psr.parts) {
		return 0, io.EOF
	}
	psr.reader = bytes.NewReader(psr.parts[psr.i].Bytes)
	return psr.Read(p)
}

func (ps *PartSet) StringShort() string {
	if ps == nil {
		return "nil-PartSet"
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf("(%v of %v)", ps.Count(), ps.Total())
}
