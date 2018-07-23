package crypto

import (
	"github.com/kardiachain/go-kardia/common"
)

type PubKey interface {
	Address() common.Address
}
