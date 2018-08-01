package crypto

import (
	"github.com/kardiachain/go-kardia/lib/common"
)

type PubKey interface {
	Address() common.Address
}
