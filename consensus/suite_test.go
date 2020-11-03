// Package consensus
package consensus

import (
	"math"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardiamain/configs"
	g "github.com/kardiachain/go-kardiamain/mainchain/genesis"
)

func init() {
	configs.AddDefaultContract()
	configs.AddDefaultStakingContractAddress()
}

const (
	testSubscriber = "test-client"
)

var (
	testMinPower           int64 = 10
	ensureTimeout                = time.Millisecond * 200
	initValue                    = g.ToCell(int64(math.Pow10(6)))
	defaultGenesisAccounts       = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}
)
