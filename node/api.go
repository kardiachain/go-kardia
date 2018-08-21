package node

import (
	"github.com/kardiachain/go-kardia/lib/common"
)

// PublicKaiAPI offers helper utils
type PublicKaiAPI struct {
	stack *Node
}

// NewPublicKaiAPI creates a new KaiService instance
func NewPublicKaiAPI(stack *Node) *PublicKaiAPI {
	return &PublicKaiAPI{stack}
}

// BlockNumber returns the block number of the chain head.
// THIS FUNCTION NOW ALWAYS RETURN 100
// TODO: Implement actual logic to get blocknumber here.
func (s *PublicKaiAPI) BlockNumber() common.Uint64 {
	return common.Uint64(100)
}
