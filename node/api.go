package node

import (
	"github.com/kardiachain/go-kardia/lib/common"
)

// PublicNodeAPI offers helper utils
type PublicNodeAPI struct {
	node *Node
}

// NewPublicNodeAPI creates a new PublicNodeAPI instance
func NewPublicNodeAPI(node *Node) *PublicNodeAPI {
	return &PublicNodeAPI{node}
}

// PeersList returns the number of peers that current node can connect to.
func (s *PublicNodeAPI) PeersList() common.Uint64 {
	return common.Uint64(100)
}
