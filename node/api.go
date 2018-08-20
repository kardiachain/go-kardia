package node

import (
	"github.com/kardiachain/go-kardia/lib/common"
)

// PublicWeb3API offers helper utils
type PublicKaiAPI struct {
	stack *Node
}

// NewPublicWeb3API creates a new Web3Service instance
func NewPublicKaiAPI(stack *Node) *PublicKaiAPI {
	return &PublicKaiAPI{stack}
}

// BlockNumber returns the block number of the chain head.
func (s *PublicKaiAPI) BlockNumber() common.Uint64 {
	// 	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	return common.Uint64(100)
}
