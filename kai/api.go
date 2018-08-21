package kai

import (
	"github.com/kardiachain/go-kardia/lib/common"
)

// PublicKaiAPI provides an API to access Kai full node-related
// information.
type PublicKaiAPI struct {
	kaiService *Kardia
}

// NewPublicKaiAPI creates a new Kai protocol API for full nodes.
func NewPublicKaiAPI(kaiService *Kardia) *PublicKaiAPI {
	return &PublicKaiAPI{kaiService}
}

// BlockNumber returns the block number of the chain head.
// THIS FUNCTION NOW ALWAYS RETURN 100
// TODO: Implement actual logic to get blocknumber here.
func (s *PublicKaiAPI) BlockNumber() common.Uint64 {
	return common.Uint64(100)
}
