package cstate

import (
	"github.com/kardiachain/go-kardia/types"
)

//-----------------------------------------------------------------------------
// evidence pool

//go:generate mockery --case underscore --name EvidencePool

// EmptyEvidencePool is an empty implementation of EvidencePool, useful for testing. It also complies
// to the consensus evidence pool interface
type EmptyEvidencePool struct{}

func (EmptyEvidencePool) PendingEvidence(maxBytes int64) (ev []types.Evidence, size int64) {
	return nil, 0
}
func (EmptyEvidencePool) AddEvidence(types.Evidence) error              { return nil }
func (EmptyEvidencePool) Update(LastestBlockState, types.EvidenceList)  {}
func (EmptyEvidencePool) CheckEvidence(evList types.EvidenceList) error { return nil }
func (EmptyEvidencePool) AddEvidenceFromConsensus(ev types.Evidence) error {
	return nil
}
func (EmptyEvidencePool) VMEvidence(height uint64, evidence []types.Evidence) []types.Evidence {
	return []types.Evidence{}
}
