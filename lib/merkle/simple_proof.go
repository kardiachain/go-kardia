/*
 *  Copyright 2019 KardiaChain
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

package merkle

import (
	"bytes"
	"errors"
	"fmt"

	kcrypto "github.com/kardiachain/go-kardiamain/proto/kardiachain/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

// SimpleProof represents a simple Merkle proof.
// NOTE: The convention for proofs is to include leaf hashes but to
// exclude the root hash.
// This convention is implemented across IAVL range proofs as well.
// Keep this consistent unless there's a very good reason to change
// everything.  This also affects the generalized proof system as
// well.
type SimpleProof struct {
	Total    uint64   `json:"total"`     // Total number of items.
	Index    uint64   `json:"index"`     // Index of item to prove.
	LeafHash []byte   `json:"leaf_hash"` // Hash of item value.
	Aunts    [][]byte `json:"aunts"`     // Hashes from leaf's sibling to a root's child.
}

// SimpleProofsFromByteSlices computes inclusion proof for given items.
// proofs[0] is the proof for items[0].
func SimpleProofsFromByteSlices(items [][]byte) (rootHash []byte, proofs []*SimpleProof) {
	trails, rootSPN := trailsFromByteSlices(items)
	rootHash = rootSPN.Hash
	proofs = make([]*SimpleProof, len(items))
	for i, trail := range trails {
		proofs[i] = &SimpleProof{
			Total:    uint64(len(items)),
			Index:    uint64(i),
			LeafHash: trail.Hash,
			Aunts:    trail.FlattenAunts(),
		}
	}
	return
}

// SimpleProofsFromMap generates proofs from a map. The keys/values of the map will be used as the keys/values
// in the underlying key-value pairs.
// The keys are sorted before the proofs are computed.
func SimpleProofsFromMap(m map[string][]byte) (rootHash []byte, proofs map[string]*SimpleProof, keys []string) {
	sm := newSimpleMap()
	for k, v := range m {
		sm.Set(k, v)
	}
	sm.Sort()
	kvs := sm.kvs
	kvsBytes := make([][]byte, len(kvs))
	for i, kvp := range kvs {
		kvsBytes[i] = KVPair(kvp).Bytes()
	}

	rootHash, proofList := SimpleProofsFromByteSlices(kvsBytes)
	proofs = make(map[string]*SimpleProof)
	keys = make([]string, len(proofList))
	for i, kvp := range kvs {
		proofs[string(kvp.Key)] = proofList[i]
		keys[i] = string(kvp.Key)
	}
	return
}

// Verify that the SimpleProof proves the root hash.
// Check sp.Index/sp.Total manually if needed
func (sp *SimpleProof) Verify(rootHash []byte, leaf []byte) error {
	leafHash := leafHash(leaf)
	if sp.Total < 0 {
		return errors.New("Proof total must be positive")
	}
	if sp.Index < 0 {
		return errors.New("Proof index cannot be negative")
	}
	if !bytes.Equal(sp.LeafHash, leafHash) {
		return fmt.Errorf("invalid leaf hash: wanted %X got %X", leafHash, sp.LeafHash)
	}
	computedHash := sp.ComputeRootHash()
	if !bytes.Equal(computedHash, rootHash) {
		return fmt.Errorf("invalid root hash: wanted %X got %X", rootHash, computedHash)
	}
	return nil
}

// Compute the root hash given a leaf hash.  Does not verify the result.
func (sp *SimpleProof) ComputeRootHash() []byte {
	return computeHashFromAunts(
		int(sp.Index),
		int(sp.Total),
		sp.LeafHash,
		sp.Aunts,
	)
}

func (sp *SimpleProof) ToProto() *kcrypto.Proof {
	if sp == nil {
		return nil
	}
	pb := new(kcrypto.Proof)

	pb.Total = sp.Total
	pb.Index = sp.Index
	pb.LeafHash = sp.LeafHash
	pb.Aunts = sp.Aunts

	return pb
}

func ProofFromProto(pb *kcrypto.Proof) (*SimpleProof, error) {
	if pb == nil {
		return nil, errors.New("nil proof")
	}

	sp := new(SimpleProof)

	sp.Total = pb.Total
	sp.Index = pb.Index
	sp.LeafHash = pb.LeafHash
	sp.Aunts = pb.Aunts

	return sp, sp.ValidateBasic()
}

// String implements the stringer interface for SimpleProof.
// It is a wrapper around StringIndented.
func (sp *SimpleProof) String() string {
	return sp.StringIndented("")
}

// ValidateBasic performs basic validation.
// NOTE: it expects the LeafHash and the elements of Aunts to be of size tmhash.Size,
// and it expects at most MaxAunts elements in Aunts.
func (sp *SimpleProof) ValidateBasic() error {
	if len(sp.LeafHash) != tmhash.Size {
		return fmt.Errorf("expected LeafHash size to be %d, got %d", tmhash.Size, len(sp.LeafHash))
	}

	for i, auntHash := range sp.Aunts {
		if len(auntHash) != tmhash.Size {
			return fmt.Errorf("expected Aunts#%d size to be %d, got %d", i, tmhash.Size, len(auntHash))
		}
	}
	return nil
}

// StringIndented generates a canonical string representation of a SimpleProof.
func (sp *SimpleProof) StringIndented(indent string) string {
	return fmt.Sprintf(`SimpleProof{
%s  Aunts: %X
%s}`,
		indent, sp.Aunts,
		indent)
}

// Use the leafHash and innerHashes to get the root merkle hash.
// If the length of the innerHashes slice isn't exactly correct, the result is nil.
// Recursive impl.
func computeHashFromAunts(index int, total int, leafHash []byte, innerHashes [][]byte) []byte {
	if index >= total || index < 0 || total <= 0 {
		return nil
	}
	switch total {
	case 0:
		panic("Cannot call computeHashFromAunts() with 0 total")
	case 1:
		if len(innerHashes) != 0 {
			return nil
		}
		return leafHash
	default:
		if len(innerHashes) == 0 {
			return nil
		}
		numLeft := getSplitPoint(total)
		if index < numLeft {
			leftHash := computeHashFromAunts(index, numLeft, leafHash, innerHashes[:len(innerHashes)-1])
			if leftHash == nil {
				return nil
			}
			return innerHash(leftHash, innerHashes[len(innerHashes)-1])
		}
		rightHash := computeHashFromAunts(index-numLeft, total-numLeft, leafHash, innerHashes[:len(innerHashes)-1])
		if rightHash == nil {
			return nil
		}
		return innerHash(innerHashes[len(innerHashes)-1], rightHash)
	}
}

// SimpleProofNode is a helper structure to construct merkle proof.
// The node and the tree is thrown away afterwards.
// Exactly one of node.Left and node.Right is nil, unless node is the root, in which case both are nil.
// node.Parent.Hash = hash(node.Hash, node.Right.Hash) or
// hash(node.Left.Hash, node.Hash), depending on whether node is a left/right child.
type SimpleProofNode struct {
	Hash   []byte
	Parent *SimpleProofNode
	Left   *SimpleProofNode // Left sibling  (only one of Left,Right is set)
	Right  *SimpleProofNode // Right sibling (only one of Left,Right is set)
}

// FlattenAunts will return the inner hashes for the item corresponding to the leaf,
// starting from a leaf SimpleProofNode.
func (spn *SimpleProofNode) FlattenAunts() [][]byte {
	// Nonrecursive impl.
	innerHashes := [][]byte{}
	for spn != nil {
		if spn.Left != nil {
			innerHashes = append(innerHashes, spn.Left.Hash)
		} else if spn.Right != nil {
			innerHashes = append(innerHashes, spn.Right.Hash)
		} else {
			break
		}
		spn = spn.Parent
	}
	return innerHashes
}

// trails[0].Hash is the leaf hash for items[0].
// trails[i].Parent.Parent....Parent == root for all i.
func trailsFromByteSlices(items [][]byte) (trails []*SimpleProofNode, root *SimpleProofNode) {
	// Recursive impl.
	switch len(items) {
	case 0:
		return nil, nil
	case 1:
		trail := &SimpleProofNode{leafHash(items[0]), nil, nil, nil}
		return []*SimpleProofNode{trail}, trail
	default:
		k := getSplitPoint(len(items))
		lefts, leftRoot := trailsFromByteSlices(items[:k])
		rights, rightRoot := trailsFromByteSlices(items[k:])
		rootHash := innerHash(leftRoot.Hash, rightRoot.Hash)
		root := &SimpleProofNode{rootHash, nil, nil, nil}
		leftRoot.Parent = root
		leftRoot.Right = rightRoot
		rightRoot.Parent = root
		rightRoot.Left = leftRoot
		return append(lefts, rights...), root
	}
}
