package node

import ()

// PublicWeb3API offers helper utils
type PublicKaiAPI struct {
	stack *Node
}

// NewPublicWeb3API creates a new Web3Service instance
func NewPublicKaiAPI(stack *Node) *PublicKaiAPI {
	return &PublicKaiAPI{stack}
}
