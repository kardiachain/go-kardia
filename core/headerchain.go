package core

import (
	"sync/atomic"

	"github.com/kardiachain/go-kardia/types"
)

//TODO(huny@): Add detailed description
type HeaderChain struct {
	currentHeader atomic.Value // Current head of the header chain (may be above the block chain!)
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (hc *HeaderChain) CurrentHeader() *types.Header {
	return hc.currentHeader.Load().(*types.Header)
}
