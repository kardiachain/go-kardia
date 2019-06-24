/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package configs

import (
	"fmt"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	"math"
	"strings"
)

// TODO(huny): Get the proper genesis hash for Kardia when ready
// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash = common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	TestnetGenesisHash = common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d")
)

var (
	DefaultChainID = uint64(1)
	EthDualChainID = uint64(2)
	NeoDualChainID = uint64(3)
	TronDualChainID = uint64(4)
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// TestnetChainConfig contains the chain parameters to run a node on the test network.
	TestnetChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// TestChainConfig contains the chain parameters to run unit test.
	TestChainConfig = &ChainConfig{
		Kaicon: &KaiconConfig{
			Period: 15,
			Epoch:  30000,
		},
	}
)

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	// Various consensus engines
	Kaicon *KaiconConfig `json:"kaicon,omitempty"`
}

// KaiconConfig is the consensus engine configs for Kardia BFT DPoS.
type KaiconConfig struct {
	// TODO(huny): implement this
	Period uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
}

// String implements the stringer interface, returning the consensus engine details.
func (c *KaiconConfig) String() string {
	return "kaicon"
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	var engine interface{}
	switch {
	case c.Kaicon != nil:
		engine = c.Kaicon
	default:
		engine = "unknown"
	}
	return fmt.Sprintf("{Engine: %v}",
		engine,
	)
}

// GasTable organizes gas prices for different Kardia operations.
type GasTable struct {
	ExtcodeSize uint64
	ExtcodeCopy uint64
	Balance     uint64
	SLoad       uint64
	Calls       uint64
	Suicide     uint64

	ExpByte uint64

	// CreateBySuicide occurs when the
	// refunded account is one that does
	// not exist. This logic is similar
	// to call. May be left nil. Nil means
	// not charged.
	CreateBySuicide uint64
}

// Variables containing gas prices for different ethereum phases.
var (
	// GasTableV0 contain the gas prices for the initial phase.
	GasTableV0 = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     50,

		CreateBySuicide: 25000,
	}
)

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

// -------- Consensus Config ---------

// ConsensusConfig defines the configuration for the Kardia consensus service,
// including timeouts and details about the block structure.
type ConsensusConfig struct {
	// All timeouts are in milliseconds
	TimeoutPropose        int `mapstructure:"timeout_propose"`
	TimeoutProposeDelta   int `mapstructure:"timeout_propose_delta"`
	TimeoutPrevote        int `mapstructure:"timeout_prevote"`
	TimeoutPrevoteDelta   int `mapstructure:"timeout_prevote_delta"`
	TimeoutPrecommit      int `mapstructure:"timeout_precommit"`
	TimeoutPrecommitDelta int `mapstructure:"timeout_precommit_delta"`
	TimeoutCommit         int `mapstructure:"timeout_commit"`

	// Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
	SkipTimeoutCommit bool `mapstructure:"skip_timeout_commit"`

	// EmptyBlocks mode and possible interval between empty blocks in seconds
	CreateEmptyBlocks         bool `mapstructure:"create_empty_blocks"`
	CreateEmptyBlocksInterval int  `mapstructure:"create_empty_blocks_interval"`

	// Reactor sleep duration parameters are in milliseconds
	PeerGossipSleepDuration     int `mapstructure:"peer_gossip_sleep_duration"`
	PeerQueryMaj23SleepDuration int `mapstructure:"peer_query_maj23_sleep_duration"`
}

// DefaultConsensusConfig returns a default configuration for the consensus service
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		TimeoutPropose:              5000,
		TimeoutProposeDelta:         500,
		TimeoutPrevote:              1000,
		TimeoutPrevoteDelta:         500,
		TimeoutPrecommit:            1000,
		TimeoutPrecommitDelta:       500,
		TimeoutCommit:               1000,
		SkipTimeoutCommit:           false,
		CreateEmptyBlocks:           true,
		CreateEmptyBlocksInterval:   0,

		// TODO(@kiendn):
		//  - PeerGossipSleepDuration is the time peer send its data to other peers, the time is lower,
		//  the rate of data sent through network will be increase
		//  - PeerQueryMaj23SleepDuration is the time peer listens to 2/3 vote, it must watch anytime to catch up vote asap
		//  => proposed block will be handled faster
		//  => blocktime is decreased and tps is increased
		//  Note: this will cause number of blocks increase a lot and lead to chain's size increase.
		//  But I think we can add a function to check if any tx in pool before creating new block.

		PeerGossipSleepDuration:     2000, // sleep duration before gossip data to other peers - 0.5s
		PeerQueryMaj23SleepDuration: 1000, // sleep duration before send major 2/3 (if any) to other peers - 0.1s
	}
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits for a single block (ie. a commit).
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time {
	return t.Add(time.Duration(cfg.TimeoutCommit) * time.Millisecond)
}

// Propose returns the amount of time to wait for a proposal
func (cfg *ConsensusConfig) Propose(round int) time.Duration {
	return time.Duration(cfg.TimeoutPropose+cfg.TimeoutProposeDelta*round) * time.Millisecond
}

// Prevote returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *ConsensusConfig) Prevote(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrevote+cfg.TimeoutPrevoteDelta*round) * time.Millisecond
}

// Precommit returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *ConsensusConfig) Precommit(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrecommit+cfg.TimeoutPrecommitDelta*round) * time.Millisecond
}

// PeerGossipSleep returns the amount of time to sleep if there is nothing to send from the ConsensusReactor
func (cfg *ConsensusConfig) PeerGossipSleep() time.Duration {
	return time.Duration(cfg.PeerGossipSleepDuration) * time.Millisecond
}

// PeerQueryMaj23Sleep returns the amount of time to sleep after each VoteSetMaj23Message is sent in the ConsensusReactor
func (cfg *ConsensusConfig) PeerQueryMaj23Sleep() time.Duration {
	return time.Duration(cfg.PeerQueryMaj23SleepDuration) * time.Millisecond
}

// ======================= Genesis Const =======================

var InitValue = big.NewInt(int64(math.Pow10(10))) // Update Genesis Account Values
var InitValueInCell = InitValue.Mul(InitValue, big.NewInt(int64(math.Pow10(18)))) 


// GenesisAccounts are used to initialized accounts in genesis block
var GenesisAccounts = map[string]*big.Int{
	// TODO(kiendn): These addresses are same of node address. Change to another set.
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": InitValueInCell,
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": InitValueInCell,
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": InitValueInCell,
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": InitValueInCell,
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": InitValueInCell,
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": InitValueInCell,
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": InitValueInCell,
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": InitValueInCell,
	"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": InitValueInCell,
	"0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4": InitValueInCell,
	"0x8C10639F908FED884a04C5A49A2735AB726DDaB4": InitValueInCell,
	"0x2BB7316884C7568F2C6A6aDf2908667C0d241A66": InitValueInCell,
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"0x36BE7365e6037bD0FDa455DC4d197B07A2002547": 100000000,

	// stresstest accounts
	"0x757906bA5023B92e980F61cA9427BFC15f810B6f": InitValueInCell,
	"0xf80927B05dc9a25247F3d68B8192cD78361C79f0": InitValueInCell,
	"0xA58Cc74A805adb80DA35925F28f7B732Bbf3C9FF": InitValueInCell,
	"0x9afD6D161E3e19c0191d83D3f48384DdBF24Ad7b": InitValueInCell,
	"0x8A618C045Cb00E8abf6A9cab60edd10dd5572119": InitValueInCell,
	"0xA6E9bA68b1C3d9b45c1a07Aeb04169465CCAa69f": InitValueInCell,
	"0xAdea9867E143445A92A5DDD11B447de59f0090A6": InitValueInCell,
	"0x5360D2e4596CB10211bef5e088A3ED705a6872B1": InitValueInCell,
	"0x74474e320Bf05D9D83B61946faC7C0c49C86e634": InitValueInCell,
	"0x3c60F22441283FDB7acdE17f1758673e7024a68A": InitValueInCell,
	"0x8c365De6Cf9644276Ce3410D0BBBB8dE06d30633": InitValueInCell,
	"0x04b7361c47432eA35B4696FA9D93F99fC7C7FeCa": InitValueInCell,
	"0x26fBc0553bF92A1f53b216bBdc9DF93F19209773": InitValueInCell,
	"0xB8258F7EA7B4C0a2Aa13a2F4C092e4a3eCf2a379": InitValueInCell,
	"0x1c9bD2E569d990c8183D685C58CFD1af948D2A8a": InitValueInCell,
	"0x90056191A8aEE8c529756bB97DAa0e7524c4b1aA": InitValueInCell,
	"0x58Aa25D60FAa22BD29EB986a79931d33A2B9C462": InitValueInCell,
	"0x6F55C53102e4493CAd5620Ee4ad38a28ed65997C": InitValueInCell,
	"0xA3a7523F5183788e1048E24FAb285439a92c3647": InitValueInCell,
	"0x96d359D752611255eAd3465Ae4E310b47a5a20b0": InitValueInCell,
	"0x737E2d1562b16FA1Bf8C7E510F0be32c0BDA059c": InitValueInCell,
	"0x291bCb8f5199Bdcd8c611b91209f1a2359AC2FdD": InitValueInCell,
	"0x595e1c030E76eF64633046A7Ca4deCcfce952C73": InitValueInCell,
	"0xF344eee5F1F689735A200244648934FEA73b6af9": InitValueInCell,
	"0x5613033C882124fAc94004E84D684504c0474C4C": InitValueInCell,
	"0x476c9e55797aB3Fe5700DD3efDc5d62333d5acc7": InitValueInCell,
	"0x00f798863bdBF323d61C82Cf5Bbc564006e90D3E": InitValueInCell,
	"0xc4Bc10Dea9CEA29ACe42A52486390A60eCD15E17": InitValueInCell,
	"0xb116638e938023fD61D81D9E1EA38230Be1c23B9": InitValueInCell,
	"0xeB431cE9b1Aa25d5AFfdb4850B2A54666355eB68": InitValueInCell,
	"0x9cD2CFa83e6d85625a64fe31779446944c024E36": InitValueInCell,
	"0x19F2A60e1819F8C5b1fB400d16c7981AE6d7bABe": InitValueInCell,
	"0xfF89F37182d98c737956eEBFD78bC43A0d43ea9d": InitValueInCell,
	"0x1D20e9405488673D82EA7fAB6fB1267eF371a8e1": InitValueInCell,
	"0x2CffCDB04c30Bc31D52032D4e797A3c7664cf94F": InitValueInCell,
	"0xD5AEF43D86ec775305Ef7A6C16487b2a8b63B97F": InitValueInCell,
	"0x62B039889D27f20FAd29e87C4d68EA5AeFcD8d37": InitValueInCell,
	"0x53a285e30D48a6CA4494dD19457E37B4fBC078Ef": InitValueInCell,
	"0x3E73Dd17D36398cf047C47178248c6ba820249F2": InitValueInCell,
	"0x06C8a5154466a17D53c40379A9ae06C4ea23264F": InitValueInCell,
	"0x8047Ea29836cB6181241Fc1fFaF59C31e2dC5eb4": InitValueInCell,
	"0xf6237F9c2565069B8C7Fbe35f7330B3BbE8135B0": InitValueInCell,
	"0xAA8159b4759f36eD00E08A62aaf84D17afFeCa69": InitValueInCell,
	"0x6B685293d2Ea983878781F7daAE1CAC0589ED308": InitValueInCell,
	"0x0e1665C86cc70C77be465a99Be5398746E3a04F7": InitValueInCell,
	"0xABDdf28B64C1Fe43753674fB62173212EA88c50F": InitValueInCell,
	"0x309674CeA993D5D35C7155AEBD302cA0ABc6e1A3": InitValueInCell,
	"0xd97FF0890162ac5044393a92C6bA2F408d58FB1c": InitValueInCell,
	"0x975DaAf256C4E9cdc98D06FbdaBc383b7Dd2B00A": InitValueInCell,
	"0xF3e4156937328f284426370BB4956f7ace172334": InitValueInCell,
	"0x13744BA2149999C15d1b4Fe0Ae19ed53698703d9": InitValueInCell,
	"0xDf987D8d4cDBf182D154401141aBAF0c17DCDdb1": InitValueInCell,
	"0xA4B5417B7DaE65AAa81cF5a3109b44d12e04522D": InitValueInCell,
	"0xa7B6dF9306cE0Fd7885596a0e9b5C637C47Ab082": InitValueInCell,
	"0x5cA86DECF53a7Fb4841cF58787F87C1743776F2B": InitValueInCell,
	"0x9d6546D4fa4E9F4167BeF19818aDac7933928405": InitValueInCell,
	"0xCFFb16DCC027318B141a70c94107a9bE46bf2235": InitValueInCell,
	"0x2e8f56089C3a257CAAA475dbD11bC31d85747A40": InitValueInCell,
	"0xE75E147b47705D262Dd1E936635C7B05567670A3": InitValueInCell,
	"0xBAFB52b89d57ae5D843ABDDE286569288070e94C": InitValueInCell,
	"0xc60182805A7299FF85DeB3f29b6927e05B8082B7": InitValueInCell,
	"0x3aE2e9D738574EF83F09e2353aEd698251D1a014": InitValueInCell,
	"0x2856532582895BAceb005FA24F011CAd5280134F": InitValueInCell,
	"0xF00CB2F0d7A40724F68f943f8c4f4a1Fe7cE06B8": InitValueInCell,
	"0xD2273F595844b66C128178E2245bef523C934986": InitValueInCell,
	"0x8386b8aD0Fe0b774B4e34B3506A438f232851405": InitValueInCell,
	"0xB0337D3eb5B6c0A872C5978c4DD6163B5c392146": InitValueInCell,
	"0xACba7B9eF6A4A9d055af4DD5650f2Df667F1D41B": InitValueInCell,
	"0xfF2cAFe6c386Bd383fE791edC2ffdA8383E4A21e": InitValueInCell,
	"0x09CFfad4f632353a8B26C014129808a6F9cdb272": InitValueInCell,
	"0x98B0106889E77cd28B12f6e359b0cDDc7CAcc542": InitValueInCell,
	"0xaDf56017d13258500EA8C7f67e5957A8E8Cca6fd": InitValueInCell,
	"0x7d4b7cFA300eCfA99a4EA8d811cCF40C271807a8": InitValueInCell,
	"0x8A93d2Aa374187a34BC03C679116d046599b6D3D": InitValueInCell, "0x24bdaA15F8D8C7889d2c66f6b8eb116eb084baf9": InitValueInCell, "0xF038bE927215E63DfE371894e818Ec96d2E51Ed3": InitValueInCell, "0x74036b7Fd50f9D4a257c0cED896Be48aFac9b535": InitValueInCell, "0x17d99DA7ec2a181677Ca2114A09Bff20C73EDb5f": InitValueInCell, "0x2214F1028Ae6C816dbB772d1333B7Cc801075a6c": InitValueInCell, "0x4Cb633Ee13B0f1Adeb5B34B2856B3e94F31619c5": InitValueInCell, "0x49E306Ef7d6bC201F54fdB4307AC79F01BfE5623": InitValueInCell, "0xF3b30813B73e35fe2068a3eFd6C3207e5f8b190d": InitValueInCell, "0x015803741Ec982b7bC8Ee658A98eB044d5c948C7": InitValueInCell, "0xf8b4FA2558aB6A881813f911Ba09E41885a87E6a": InitValueInCell, "0x99eeF578CAB154051Cc57F35945d1CC805c6f94A": InitValueInCell, "0x5af9Fa9DBB707A4Bb0d90A37F8b09E03a855d0a7": InitValueInCell, "0x6a61e9Ff463AC20520dd8cC41B9D87C7CE562200": InitValueInCell, "0x9F71cf60066d5124ef311a78acc6b25aD5098f06": InitValueInCell, "0xB65a35CD011447e5ec6FA547a44Bc8220Ad096f0": InitValueInCell, "0xbB34e5a85587b0056C2c453A9007665F0901EdA1": InitValueInCell, "0x4ec05B5fA8b653019D9407A3BaEAfDcA64f3A31E": InitValueInCell, "0xEb4DA510A36bBEBf1401454C3a0032A3ec3B45b1": InitValueInCell, "0x7087601f9CDEd0D6f46e0E8E76b0a0DF869e3df9": InitValueInCell, "0x306db66e05E7C63a23736C37541d44FD62F70896": InitValueInCell, "0x1e192508AC30CA345FF82eB22Cd6675C953A0ef2": InitValueInCell, "0xF75B59f1C90483097d2e7E2c6807762d7ce53EaA": InitValueInCell, "0xadE86b82A995bE5f09F9576F68B2e60cf335adFf": InitValueInCell, "0x9A6A1eC85dE0473a55303e4246F58fE924119AD8": InitValueInCell, "0xFc1a7D720Cb65fFFc74cCe5C2E9D71681Fb3C0Ee": InitValueInCell, "0x1F9f1fF2744cb54B196dA423368576D186527253": InitValueInCell,

	"0x3912f280C8d9bb0656ce7cA2412E15F2872e9390": InitValueInCell,
	"0xC30eAa67ECB6D14617c79C0C7aF1ecF94B35c1b5": InitValueInCell,
	"0x454494bA1551375091cF9Df63FfC6C3e80D3f245": InitValueInCell,
	"0x56C466293ce0182a1Cbef02C40929A54e489Eba6": InitValueInCell,
	"0x93A67DEB58aD74aCcdE34C724c26901A21DAaE5b": InitValueInCell,
	"0x209fFe041aD149D62AA22F0e7e609F56d7C8299F": InitValueInCell,
	"0x2212b13Ed0DAADfB79B7b86ed29ABbfDA0b59F12": InitValueInCell,
	"0x8870c76ffD3763f106f3cb1cB7d13bae0d7Ecf88": InitValueInCell,
	"0xaFa3384beBb80F8EAB0dB6fe9E87Cd5ae7496c9c": InitValueInCell,
	"0x8CE8708075edd96570ac68AFb60196B1bbaECDD5": InitValueInCell,
	"0x23f7e9a64B26787c06CA68B2a258f759E4F9a4d4": InitValueInCell,
	"0x275Ce4Afad60B0D3D4AC75656398107C2e916E00": InitValueInCell,
	"0xb95fFAa9281eB45e43106De70B66E4134A5DeD75": InitValueInCell,
	"0x545f17337980A46953F571f5EbF6B71994411DD8": InitValueInCell,
	"0x55541DFC9F554275CFDcC83D2fd94Ccaf6fF602d": InitValueInCell,
	"0x21A8EB4eAC0b2dD589EAE47FDaD182EB6D2d7dd3": InitValueInCell,
	"0xa894CFf793F53C76853b7a4e3Ec97F4E4f17FE51": InitValueInCell,
	"0x75E8c0D6d368462AB93c775A1Fa960fee3531ddB": InitValueInCell,
	"0xa109138bD5A7c77081DF75a105e4B11e89BE882e": InitValueInCell,
	"0x0512d6cE79D5922c048caAAb814e9480e78C10a6": InitValueInCell,
	"0x8511D1c67427fa6199Fd462D00eE4C1658c03010": InitValueInCell,
	"0x12454E679130F4935Ea240Dc53ee8a9D851f19a4": InitValueInCell,
	"0x6C83faBA9ff8Bd36D147f158795A0D575047B361": InitValueInCell,
	"0x26F8c1bA1B3259c0479a95F759A245dAf6249127": InitValueInCell,
	"0xD0bfD16E2bc10c9a8D41F9931A633fe66Db4FAE8": InitValueInCell,
	"0x652418D8eB4bdc915b378166692f6a3C9f66E394": InitValueInCell,
	"0x7D947416382828560b6db6fBE3596EEBe3C60c80": InitValueInCell,
	"0x233184Ae533849D7C61A2Ef78683ACAd30B918c5": InitValueInCell,
	"0x175403013D2E77BB91Feca5CBeCbCe5E5A919920": InitValueInCell,
	"0xeE1A38f9F72811ceB5D3397354E9BE1eCDC1aAEB": InitValueInCell,
	"0x8C8Ff7121A4d3858Ab7AC4ce2FCb144ACE90a376": InitValueInCell,
	"0x945FC459d3796035074Dd8271d9257029a19c610": InitValueInCell,
	"0x2D69f9D718D76849850B4C892b5c34991f9644CE": InitValueInCell,
	"0xAB6b817dB3F42089b5338D2b813f83208D8bD095": InitValueInCell,
	"0x248c509a937400b9811Cd5952eb458A713960C8a": InitValueInCell,
	"0x496Ff51362f0f2ed218caf95FAC30e582a958b5d": InitValueInCell,
	"0xe69Fe5D10CeEe236c72F8f73c08A174e8BDA04da": InitValueInCell,
	"0xef9D8Cb340A0Dbb1F5ffDE9aAd882B0ab83D2778": InitValueInCell,
	"0x5f08b4aba7e1e1B5C284475b5c56A3eF030f3465": InitValueInCell,
	"0xC74887eAcaEef30378E1842C55Ddc8A714FAd37b": InitValueInCell,
	"0x184D41132c84f67c85Cc355058EaC90188B80287": InitValueInCell,
	"0x2C0369DF113f65650E50CC5f96C74a0cDba567a4": InitValueInCell,
	"0x39474CcE55f9d10005AE10BB58d86A5a9332527C": InitValueInCell,
	"0x2E3c2A38BaB95766bfEE3f6ABE9C306cCA69EE8f": InitValueInCell,
	"0xD29b82C845007ac4Ea871FdDf0BC89F7BaaDFF66": InitValueInCell,
	"0xbf45D3D5894080BAe939261ec5ECE383d1B86933": InitValueInCell,
	"0x4F1274d4697D0457d91Cb607f5B6981A2528D79D": InitValueInCell,
	"0x21CA0f6EDbeB87c22B180233E0D7DE5a011c49B9": InitValueInCell,
	"0xc2E76121CFdaB0ea45831d5E9EE060B491B94248": InitValueInCell,
	"0x8A119F3D82d4fc6872FF8bA1dea5DCd4beA229D0": InitValueInCell,
	"0xDCffa0C83180a0DE0E374F02998D80fF72d03b8D": InitValueInCell,
	"0x45F14EB98c2AaC73adE03c2172F4A97527888dCD": InitValueInCell,
	"0xFFa98aa4E68F8784415762615708Bf08DA72BFa9": InitValueInCell,
	"0x7092588cf39599464C8DA2bf917e671572CDcc2C": InitValueInCell,
	"0xe1a6A1d44300159Daa6255d411e5F90526b68369": InitValueInCell,
	"0xec260c042cf10062eCdaAf4D7ce071c34D09F0b4": InitValueInCell,
	"0xC4D5595Cb104e423235380D485154eAa2E6605Be": InitValueInCell,
	"0xa751E49F4eed8609E9014fD240A2e9A5FF52434e": InitValueInCell,
	"0xC41824658A40cf5a79785b30dFf0F0f72eAf44c7": InitValueInCell,
	"0x3eA8E2c5067b6684a5e354445E79De0Ea6d3cBA9": InitValueInCell,
	"0xb97cc403e5b8c4C2BD1229aF56875971df9aDBeC": InitValueInCell,
	"0x95896fFc832f64c81679cb2Db5ECdE1b21C571b9": InitValueInCell,
	"0x29927eE1765bF021dF27aCc3C7654d84ac27b177": InitValueInCell,
	"0xf3A6bE146E96bf668Aa7Ad86a00E94bBdC1e3E36": InitValueInCell,
	"0xc0d43531bBa86cD8041C8ac995646A1443946D11": InitValueInCell,
	"0xbB06BF3E8f6e866d9d192663083704321CeE5a48": InitValueInCell,
	"0x08C67737da87b7A58308A5354fCF9175a6bFcc0F": InitValueInCell,
	"0x9147c6e631b78B9C7623A847E3392589938F22bA": InitValueInCell,
	"0xeA8D832A14179bDD6884eE0481804F31E9E46D50": InitValueInCell,
	"0x10ecc59b7E06D13bE3fc30855F771Fa68d826F76": InitValueInCell,
	"0x0bE089Ab406B7becf34b3C5B5b64d26702ED57E8": InitValueInCell,
	"0x41c415262667496bE6B86329aB70258EBf4b88Fd": InitValueInCell,
	"0xEEFbD5eD52481e88e358E08E4b3F908174eC881b": InitValueInCell,
	"0x49722356FB53D7Cc7cc713C919D538A3B4818B87": InitValueInCell,
	"0x0469F79D61Fcf88e7a5a6fC15b466f69f39CbF47": InitValueInCell,
	"0x28d18554f1621cE2fad7aa5cAC512ee11A8b766F": InitValueInCell,
	"0xa624B51e365dcA3f1B96F741A853a282741c4f0F": InitValueInCell,
	"0x9ca0BA8d38eB8E0b7D4ec0dDBcD5E9d5fAdCAbD6": InitValueInCell,
	"0xf39d6c14F159504B20A8868Aa3572d458A989F90": InitValueInCell,
	"0xB420ED62853396ff3B67332D77b96D3D73B74355": InitValueInCell,
	"0x465D3b00cbb61b7901021179Ee6Ea684B6CdadF9": InitValueInCell,
	"0x162eFcC902a8Cf55C902a9e2a0CAe0D7B0B8E463": InitValueInCell,
	"0xD688667B62B4D5251095A8Fc773e3847A6FC38a7": InitValueInCell,
	"0x9eA15621CE809181E93815075ACeeb9FE1e106e7": InitValueInCell,
	"0x78bF37Acf2360E8ed6bAaa384202779EA0df472b": InitValueInCell,
	"0xBFeDC74482D2a522C0b45E798E32065baf28ee2B": InitValueInCell,
	"0x2d34F30b80687290bB7077bfCDcF1250e067D429": InitValueInCell,
	"0xBA4B14E4D9a82409369443A76BE4C5dCCC517eB7": InitValueInCell,
	"0xf73657DaA3DBdfEc68EB18fB57fDc323C26598b2": InitValueInCell,
	"0xEa4D2A1a8358156C1e2B6a58Ebf4EaF270CA6ce9": InitValueInCell,
	"0x8439705461C0E84f05dB56B3F746b67056a6dbC4": InitValueInCell,
	"0xb3bafdB3020070A3e08D43eA57bD185f91083ab9": InitValueInCell,
	"0x13603d5420224A10A5818fa423aB21C0fFEFE869": InitValueInCell,
	"0x9ceD9a019F054476b97c5044D48C4480F26abDAB": InitValueInCell,
	"0x5681099b18D430830d6c68049375D5c30F981Fda": InitValueInCell,
	"0xf2fb843396f971b8299619e56440B82495776a0F": InitValueInCell,
	"0xDFE8FFcfB693A4AB9ff10862b4Cf216Ac077129F": InitValueInCell,
	"0x4139A7DE73C1E9C04F873C5311be5ebA7b51a10b": InitValueInCell,
	"0x1c019eAB8C44F18bF32EE3f1Ee32b5eD93E8Fa36": InitValueInCell,
	"0xbEdd2694361ec0EDb5ac99E95f44EDe578A4Bd89": InitValueInCell,
	"0xD39af14644165573dda5a642DfE215442C559F65": InitValueInCell,
	"0xf382b626c0ee59d9A281537c121D307C7E524C00": InitValueInCell,
	"0xf967365a67d8D4107bA1b58bE7136bf9C5DAFa6c": InitValueInCell,
	"0xAaB6A9b24DD05d61E621590326B53443955B6437": InitValueInCell,
	"0x3f63Ec11e9d6A4a1005fA86139831185E1a0Ffa2": InitValueInCell,
	"0xCc7Add9B29091A37f9513628A15f686D7cdFD4b1": InitValueInCell,
	"0xB535c49DA28a3B3B0AdF7cc49420fDD660EB7fc4": InitValueInCell,
	"0x8b1a96aa5F29d076baA07bc343C79BDc026703F3": InitValueInCell,
	"0x5900C620De82FAfF0a1bC483b92cA18AAfE6C344": InitValueInCell,
	"0xA86bFdc8B7025Fd536cA4E2ff9501022b82D2319": InitValueInCell,
	"0x5e1D2365C151aD8360fc951A7B8D376363CDFA7C": InitValueInCell,
	"0x24201A9f9EA8ad95c4e6d41Ed5897B6F1803be34": InitValueInCell,
	"0x4f709Fe2feE5867043e3e6dC9D8c333d766335E2": InitValueInCell,
	"0x7954978a42585a1f8f42033211c4B10669101434": InitValueInCell,
	"0x72E710Ca2faCd748c8b6d8e3384A670d9E4db37F": InitValueInCell,
	"0xcd29Da93dAcb42c159bD20CC101011BF71d6665C": InitValueInCell,
	"0xEE07453651943A279F3720E90EfaE4B8062fCBdc": InitValueInCell,
	"0x4DD3a894eCFD053d43fa6907D87a3148D096F914": InitValueInCell,
	"0xa002eAc0E0C7f0A04aaCAdf525D15385e62cbD1c": InitValueInCell,
	"0x8Dbb0C85766B926440Ad86B65661909a971619D6": InitValueInCell,
	"0xCe677D240f47CE9da4ad62FDdc300D34FF5dc570": InitValueInCell,
	"0x022123Ea2beaC334d1C719dA3c50f115d5D3F66A": InitValueInCell,
	"0xDDB94ACD7e6882A022A39697bf5ac91aeEbA1789": InitValueInCell,
	"0x560507A3b5bD55494Bc77fAbF5b85D63a0d78343": InitValueInCell,
	"0x57bf6342Ce7a204dbB4516277C411B9A2769Dc30": InitValueInCell,
	"0xEd5a978dDbD00f2e419b06a1Bf0A89a969B1f01F": InitValueInCell,
	"0xe3831bBaa2A26C81D9Ab5f7580420196c8d2cC02": InitValueInCell,
	"0x138c3D416a96a854E6EB138e55Bbb1BFCAAEed56": InitValueInCell,
	"0x17E86AcA69C12cDA9519e858411CDBC12D79F4Fc": InitValueInCell,
	"0x594Fbfc65EE9adaeec98b9bA1D89Ea75a31AC1b5": InitValueInCell,
	"0x1B7648833aE1E668801252AdaCC98BD55252A079": InitValueInCell,
	"0x98B50b2B8E8e7F55bfa51652806Fa477C88DD35b": InitValueInCell,
	"0x30152a7D5Eb3a07fC3Fb47100e4B1e7dE0C0DB53": InitValueInCell,
	"0xE3e60fE8F90283F6773A2D9f59a1f53165Fa2E4B": InitValueInCell,
	"0x4495709B8c523E64951E79884e09577fA244d4D3": InitValueInCell,
	"0x9561dF9D60edb6b3AB2e55D9DD0aA2b1B249E30c": InitValueInCell,
	"0x73148af2189ED7b6C83D070e83DC3C63446B0Dbe": InitValueInCell,
	"0x8319E9AfB49818f9279404DAd584743816FBb8b7": InitValueInCell,
	"0x97a8ea69059b0eaB9B44347A71FE64fff4B1D026": InitValueInCell,
	"0xb6f19Db2117004C2bbC741A401bFDa1A0b67a908": InitValueInCell,
	"0x92506572e7F54C785048Af7621198597E06E556B": InitValueInCell,
	"0xf4F5CdE076b23D6b6170e609B52A9dfd2c8B1F7f": InitValueInCell,
	"0x2A535009CD7Fc9f0d689176F0b6b865489421b2F": InitValueInCell,
	"0x108aeD8B1fc761B40Ac10F6699bD7D62130F12B2": InitValueInCell,
	"0x44353a99D102014160685D815275efa48DFcd120": InitValueInCell,
	"0x7D78fd7dCB4098BB6A1346117435425375E3c274": InitValueInCell,
	"0x8AA28B0EBbddb233cF8631131F5d737d9FFD25b7": InitValueInCell,
	"0x6f9548d11a3028Cf92b9a25819E38bA39CE07C2B": InitValueInCell,
	"0x34A230BC38e1dB04E3A8C6690D40e9B34c973457": InitValueInCell,
	"0xA0EF2A72E411247756f53E8308745a2DF3E899dA": InitValueInCell,
	"0x84Cb913fa39C48DC88f19eb5f02a9Ba85d46aBc6": InitValueInCell,
	"0x7Af0a590E3bd9C2333273a83f43184691C2cA865": InitValueInCell,
	"0x117aD7ca15d44a62A593F8ce586FdBea3a92E3cc": InitValueInCell,
	"0xc31D73DE913C27B412112deec62152815703B840": InitValueInCell,
	"0xF4a1A66aC7f9dec3335de0AEA48A7395297798Bc": InitValueInCell,
	"0x9823A474E3D2E0260899270FBbE547467B0ef0B3": InitValueInCell,
	"0x4029a3fAE2aE824df9D32464157c1ed166c94BdB": InitValueInCell,
	"0xFbE134A006d8c1eC2b2142BAAF040ed04E5606dB": InitValueInCell,
	"0x7EB18F9Db4De41cFBF720228FB1B6150dbe14479": InitValueInCell,
	"0xE3E22Cd75751E7876Ce51Ca33A6E83d515442A1c": InitValueInCell,
	"0x3A0a80d4D9e27CFA61D725d2F50aC8DB67ea9B97": InitValueInCell,
	"0xd8e70997a7Ec878CfF180F895Fd1Ddb02241C543": InitValueInCell,
	"0x6A52c724184Db5fFd8cabc5251053e5eEf5515b4": InitValueInCell,
	"0x3C21f496b369F94a6a0ae369Ea8520635aef08a4": InitValueInCell,
	"0xBFaDF9a1E1DEfA133D7ac55724F456031Cc21F2c": InitValueInCell,
	"0x93146961129e04D7359f931932517Dcca12b9CAd": InitValueInCell,
	"0xE0281910eABFFfC729b9086Fa1F3Aa6C786313b1": InitValueInCell,
	"0x0235E556118f951016B940f14f589051a9E9586f": InitValueInCell,
	"0xe1c622400Cb9E858CDf5D53aF65d7Bf6CC9ca1D1": InitValueInCell,
	"0xcDE9a270eAF3dfb3689380dDC2d1abe0717E7922": InitValueInCell,
	"0x1Ab599477876f0C12fF7784B709de16fADfA4F0c": InitValueInCell,
	"0x5b6473E6264c1D81e40ed60C1f631b9dF2022A01": InitValueInCell,
	"0x1918eFD07A6CbECB33f573c21f879eCFDF1b5a4D": InitValueInCell,
	"0x481D0DC4f5E2a16f6216E6F6E00E1AB23332230A": InitValueInCell,
	"0x65B8AB4194AAf57F8f290E39643766F19cCC8451": InitValueInCell,
	"0x22804d018B28EB93aB7dd1A45e04176109A50fA9": InitValueInCell,
	"0x61F666a0b0fd05D2014c77fb4d7825c8A026eBff": InitValueInCell,
	"0x47F764EBFF4e432Ed16fa8E8AfE10cD54339D097": InitValueInCell,
	"0xBa57FBf6350856f387745B3Dd0DeEE7acEEF2151": InitValueInCell,
	"0xD4DBf3a2FC399B232BfFF3ef27439aD98cc86E82": InitValueInCell,
	"0x403Fae3D8DE5d5FAa015544A9ffEDEdB7abdFa7d": InitValueInCell,
	"0x6190B8eC760c15688aaebB0225E15ffD0D736aa6": InitValueInCell,
	"0x43c958e3F1986E9CD1FB18b2ff3d8C9b9d8f118E": InitValueInCell,
	"0x8df554bEE5d4a2B1501F3a7aeFA2496CD22d2B59": InitValueInCell,
	"0xc1e91eF0c716fA3E1889A40d99bd4239324A371C": InitValueInCell,
	"0xB6e979b9e9f5f3B72861A2D97401ca4f2cf1b527": InitValueInCell,
	"0xF6da2843A6b5e444fB2F4F855c519B81756d6ccC": InitValueInCell,
	"0x6B614D423d3142c87a7420013740eDe1E622a31f": InitValueInCell,
	"0x20B808694cE764823Ae193BD6C60d2076de213DE": InitValueInCell,
	"0xeb3F0241f301cd336E5F364ABF5F8d48c8719520": InitValueInCell,
	"0x707D95b0AAE1C02f3b41cdC86f93C81C08846018": InitValueInCell,
	"0xFD5de46aBD78f23576715c1f035f30Ec64e324fB": InitValueInCell,
	"0x23dccC1BDaAb86b082FD481b954bDb3E0D152C04": InitValueInCell,
	"0x58C6966f739d3b6A747Cf80A506927C75C0e6E61": InitValueInCell,
	"0x70d12EDE9d342AB5D919CFdB3c79b90e3D760249": InitValueInCell,
	"0x925628e229BA8e733A5003d53C89cc9A2cE00Baa": InitValueInCell,
	"0x926DEaacabbdC50856c82f385dc2b3B00b76401d": InitValueInCell,
	"0xeF9B41B1124063a9Ed92aF087224c21f00d3c5f0": InitValueInCell,
	"0x95a0106F3aAECC8A11413b398671aC9Aa0D95D42": InitValueInCell,
	"0x7A55421145B95a655bEF5bB3BF98da1459411009": InitValueInCell,
	"0x86E3efd723a75E24477FF5C7ed04BaA26dC7C413": InitValueInCell,
	"0x4062cBE8516a43f6DdF496eA1D1bDE1AbE74B92b": InitValueInCell,
	"0xf9099a0C981a2369a67260AbFe648Ad98b175Bd8": InitValueInCell,
	"0xA0375d6B5EAa0d926e67312604C17AE1D97E6a87": InitValueInCell,
	"0xB10CC4d174c500bcf183ad0d8041BC15e3475726": InitValueInCell,
	"0xcc59CF85AE5F322BfCA84D86B422614a6B4B3386": InitValueInCell,
	"0x193E9126363253804551C501a9bBa650308Dc0ba": InitValueInCell,
	"0xD23F4B9a9f11903cbdC35076838457527C73dfCf": InitValueInCell,
	"0x7425399E5d0aa9Aa6875B362240f09A1Fc570474": InitValueInCell,
	"0xE052B3a61460dd401B48396Cf6a273D53Ef8eD9C": InitValueInCell,
	"0x0ce4bE64CB6488Ec44B847aebd61147a1cbEcEb4": InitValueInCell,
	"0x4aC865f68633660c8bf680D0979344646be20BA8": InitValueInCell,
	"0xdF53c52a702b6ecB21458C54E3187E69Db4b4B71": InitValueInCell,
	"0x348A2F1845fa37D357ae1cF9EcFF64F86306F5c4": InitValueInCell,
	"0x58ba21471f5F31c65665EDd4e13fEAF98B6187c0": InitValueInCell,
	"0x28C1727bD07bfb0FFfB3d6B8E68C51e30c072B2d": InitValueInCell,
	"0xaE499246EDF0fb56f1084651BeFd48bE1CEa971A": InitValueInCell,
	"0x6a137dF08835f0DD4D145dDA259124B0C85d6fA5": InitValueInCell,
	"0xa154de2dDf028fB429A9da97dC3C801aA06dbf68": InitValueInCell,
	"0x475Fad7CcD7E21099d3e2C4f51BBf8954B7B4a14": InitValueInCell,
	"0x0F7D4042299563c360852A3d0A502a7cC6C0ec1d": InitValueInCell,
	"0x23f9570DB9f0a49BC65F2280FF0D48815C7e703e": InitValueInCell,
	"0xd87b51A274e435920f9b4655C597693eEFeab873": InitValueInCell,
	"0x8acb50102362b66a728338Af200ab0cEbC934043": InitValueInCell,
	"0x89FddDEC85A08957492A5A8F70DC5657386Badc7": InitValueInCell,
	"0x23b21e075E2fF447b931180BcE1a227b55aDeDbC": InitValueInCell,
	"0x89b74Ea833FF7e57F5BB3A73c1e7eB1F8248D2F2": InitValueInCell,
	"0xc9F654629819216A91093a579af88a3ea8FDa9Bb": InitValueInCell,
	"0xf00f518ED3a71588E200BBDBe9d67b854416A08f": InitValueInCell,
	"0xB59Bc089fb568c6dFCa37648A08658A2131c5E9c": InitValueInCell,
	"0x18a3171FDc37B633b5e25304E829535827c5BC43": InitValueInCell,
	"0xdF9FaFD68Fc08c605E96F00166C6C13c7DE1c281": InitValueInCell,
	"0xe523dc495a0837e71Fbcd008D1D5427880DCEb67": InitValueInCell,
	"0x1CaF3A8b3306A126885f1ABC7bdE376cE08fF520": InitValueInCell,
	"0xbA1815fcFFEB8fFc18591dd5587fC6CB35F244c2": InitValueInCell,
	"0xb335014AaFd88110CE3Af12B6C0d7AF560336827": InitValueInCell,
	"0xBC37B4d5e5e562917a598260cd28759da3D89726": InitValueInCell,
	"0x22A38c0fCDd270f9613250f66eD4b759E7dEC5D0": InitValueInCell,
	"0x09887A85A68bd7089F5bA326195d3fb43E227dB5": InitValueInCell,
	"0xb4C345B5E46f86D7f8c4dFCC595568CCca3972Be": InitValueInCell,
	"0x041cA2af2b3a5A912486A22b3e15C690d6f681EE": InitValueInCell,
	"0x210dA2612B45B24cf4a98A1d63E2c4e3eC571530": InitValueInCell,
	"0xeA343Ab026bE8418194853bF79D55dA9db654bD7": InitValueInCell,
	"0xABb786e04fe72f66DaA61C1C7b4ae1844753833b": InitValueInCell,
	"0x35662fBf43E04Ca8873580bDBa77f5eB00aDC36B": InitValueInCell,
	"0x6862017A9a4fcdd54954637c38Bb9906FEeaE1FB": InitValueInCell,
	"0xe4a85894F36F131E076B2C6Ff1298E5424C75F9c": InitValueInCell,
	"0xeaAd9dCb3F2c568dcA4C7BD20EaF1A9DF7201721": InitValueInCell,
	"0xF66538EB557c32eA9dde18fAf425047f45C819D5": InitValueInCell,
	"0x693ba8f8F224B062d1Bf3509c719B579786592c1": InitValueInCell,
	"0x89eB0820dcB33F17a8c13006AA379BB9874B8545": InitValueInCell,
	"0xb91230ccF167628Dd1bD0B26A6307826EeA669C9": InitValueInCell,
	"0x7f5A23F60C5617FD5D99CBc5703ADD6e52461643": InitValueInCell,
	"0xA7e1e4B6a390dFF1eF261791f6D483882203A375": InitValueInCell,
	"0x6cfB73EDE12D3C1b98f233acD42ED0C5CdC1617F": InitValueInCell,
	"0xdD730648cddbc25F76fEeF55b18f0e5aCFa8FB99": InitValueInCell,
	"0x83B8494566d0b41385d4892394eABF6b124bFd7F": InitValueInCell,
	"0xC49635c73705AE6a5f03D03Fd6e56475e36124e1": InitValueInCell,
	"0x80293C6cBD4637D11C4cB3356027bbeb24da5CCB": InitValueInCell,
	"0x0b5049D03b8F45BD6EF4a0A517bC781E0634B7f4": InitValueInCell,
	"0x3Bd0d81ba5b59306a2da9E9f16905f970c798E6F": InitValueInCell,
	"0xA4B24e023C54EF1EB079c20f5C5cfB9204C8aD58": InitValueInCell,
	"0xFdEf0e90d0647Cb17C72857cC54b4A30D4BE3F71": InitValueInCell,
	"0x9Ac4816366Fdc47E8dA98bc448185F1184111B4c": InitValueInCell,
	"0x6742E24d3164a7D04099cA6bA18b45DD22c867Bd": InitValueInCell,
	"0xC560147250c296Fe3827ecF57F08439B80Dc122f": InitValueInCell,
	"0xC8b233F1E07BC77Ccf20019c53B498CD49Af9161": InitValueInCell,
	"0x2A7cbF06759905ec09E2aa767A4991D6fab4B072": InitValueInCell,
	"0x654F1DF238817a9fa0b60c0A2679fb6fE77E9637": InitValueInCell,
	"0x2248a2088519566dbFb126dD03cf185fEE30C8cc": InitValueInCell,
	"0xCb7E99754698d1108710F913f3e87F25C3482982": InitValueInCell,
	"0xA5938FE5843930f376649C55654465406C6E8318": InitValueInCell,
	"0xad30e23326492Dd2F724753D096Fd7683e97914a": InitValueInCell,
	"0xc954560EfF9a9Bc5A11ca49BBC6e53164c8271Ec": InitValueInCell,
	"0x8dcdf6591a00277212F5aAf436516106E3706622": InitValueInCell,
	"0x4178920f33a7614F07C505e1F5ff9B96Cb97A2E8": InitValueInCell,
	"0xFFcFC776A0cc21B4D8Fdc9988C5a905dC5c31353": InitValueInCell,
	"0xB8Bef98f31F6D2D018BA9Ad70E5c454B972ea0f2": InitValueInCell,
	"0xd3eCE5d303C7E30BbC3de02C53b6Fd0993f47Ed1": InitValueInCell,
	"0xa30c17fFC046B73e32EE23aE0DfAA169B19d294d": InitValueInCell,
	"0x3f012ebd9ce6399BBA502e1f0a1996a65084Ec05": InitValueInCell,
	"0xff88C6f9D0dEc61795D294C56FBdd082FF224981": InitValueInCell,
	"0xD952f3454c873CE4FD92B0cf6D8e8dd6561387aF": InitValueInCell,
	"0x8E429BebDe4E4Fa8Dfc50320464637bb21bA553F": InitValueInCell,
	"0x7F9021A371b6F48745eDc74b18cb8F3e3cAFEE5b": InitValueInCell,
	"0x6dc4cA1482F7Ccd26dF763244DaEf5b1117032f0": InitValueInCell,
	"0x19389C8F0B004cCD80aA259e1F66F953663C77BF": InitValueInCell,
	"0x4637Ff06a1edc71F530eED873AEbde75595C9F7f": InitValueInCell,
	"0x6b8Da59289372eFF5e022a7728B06A87e964FF54": InitValueInCell,
	"0xF850CDA1E45E86fbBBd7A5Eb6153a4F80715a3F5": InitValueInCell,
	"0x34e7a630ec6b8cea578ceb807838f8408Dcf0B49": InitValueInCell,
	"0xfB93d7272245e394DBf3883cf238799a593dF1c6": InitValueInCell,
	"0x05660c24B1e4650375eC60B9482e38Ad03fE7308": InitValueInCell,
	"0xE3523cf964460826Cb64e5F201686B98e5D7A9A3": InitValueInCell,
	"0x67380149f28EDe9BC8d835A42A3472eD0CBD3ed3": InitValueInCell,
	"0x45F459BBe2bC6f10B88142dE92b12B1E2B1777E9": InitValueInCell,
	"0xb1832cfc4f964ec532F488231c929B5fc73199Ab": InitValueInCell,
	"0x79AAABf09d8Fa74B592d55Cb53E724eB084AFf0e": InitValueInCell,
	"0xD3F1591df8AcA6d151B2845Fe6e28a14c03De112": InitValueInCell,
	"0xe4F96004B59752fdb8d5EC76B6225999cF905C47": InitValueInCell,
	"0x5228abE14e84A93C0f6A6d68D75a9D56eC6c34E8": InitValueInCell,
	"0xB300f8B1B58173128c7089412d924EBfEaF2213d": InitValueInCell,
	"0xb028B14f3636ef04896d13F105B55DAfAF69ad6C": InitValueInCell,
	"0x60EfdF9c74EB529034d81E08860e8277ed530447": InitValueInCell,
	"0x7cD2cb3adDd50731Dd6bbe695358FcCAd3FFC49c": InitValueInCell,
	"0xb2c9F95e13D2c7468329802C6c3ec42ff55EB987": InitValueInCell,
	"0xF3cfC404ABc52bd8c5b0fe083036bf36c7c7c1e9": InitValueInCell,
	"0xe130C2cfE9e8a890D13420b0E0F2a6fc398158d1": InitValueInCell,
	"0xA1da50f7C51afc74e83c85b07fc81659Eb473df6": InitValueInCell,
	"0xB7005bd9f050129624c852239c1027f0EB3C9490": InitValueInCell,
	"0xD5028A7E2Cc20b4b1417fe61aFd0FbF8f3f668A9": InitValueInCell,
	"0x4c9F2C42e76403f02036125481EE3fb9A5a6F3af": InitValueInCell,
	"0x6f4aD992bD7452dA6Bc04b49eEF4fF56aed80A20": InitValueInCell,
	"0x0FD0E675fB245A92b6Dfda8d6a9C1F53A974627c": InitValueInCell,
	"0x4331222474cC224821A9B5057F599E45fc71f13A": InitValueInCell,
	"0xBDEC05D84DADcA94586d254b25300fdfaA4f6a32": InitValueInCell,
	"0xcDD679400202044588d3710F333C2447aFF0b7f4": InitValueInCell,
	"0xE3a894E2cD3c265ccDd92AdEa4bD049F421f331c": InitValueInCell,
	"0xc79C647d43b1b7f52bbBD67010c7687071b5972a": InitValueInCell,
	"0xc2172E792A0419ed985246F550905c1EeAf418AC": InitValueInCell,
	"0xFC0311C99d19aB91ab1d673D3703a96E646a058C": InitValueInCell,
	"0x2e3AA386Eb3C35dFD9354dC6bCdf8a69Fa17B1f0": InitValueInCell,
	"0xEfD973009B580efC83025eA4CD57F173292AFbE5": InitValueInCell,
	"0x43BC478daeD5986B6B6FBC5208F39e71D764ea10": InitValueInCell,
	"0x8C8C9CE45b73C4F28CC5d6B3E47151fC501b8385": InitValueInCell,
	"0x62201103C4ea45129114979a478166a6fC90f0fb": InitValueInCell,
	"0x059bC0d009E42731eF093866909C6E0828380667": InitValueInCell,
	"0x71aBfcc9fFc52Ce89D6d2346a366d8c1689e5f34": InitValueInCell,
	"0x97D7a80732C778FA847B038F0B10ECB4647D3ca8": InitValueInCell,
	"0xE688D2960b8e7a5F67e920C6A895aa15edc0f196": InitValueInCell,
	"0x70802BCB7e132d351d4852D2EC54eD997D500973": InitValueInCell,
	"0xaFb2f647F4B7580BE588fbff8eC30d238B7b132B": InitValueInCell,
	"0x928Fe7a442Fb5be198A7062D0b5059b5B7E75111": InitValueInCell,
	"0xb441C2C665eC4af34D92b699b4F26a42a8041717": InitValueInCell,
	"0x43060d366921aF2009E294Df025d1c774b951039": InitValueInCell,
	"0xBF898F4fBF032F84989c0BFf289F7B62fB254C85": InitValueInCell,
	"0xe440c505101317BF3Fc725F51E663fF56aa91b79": InitValueInCell,
	"0x82d389AaE6c9d70A1C1b74f723a4d6C5DE375Da4": InitValueInCell,
	"0xEb6E053C68bee56223E49927d7713E37Fd717e78": InitValueInCell,
	"0x3d1ED940D2C38206F25a7DEE45832B85535cae7d": InitValueInCell,
	"0x610699A54024464F88A4E6bA7E3F228169811e2D": InitValueInCell,
	"0xC54640492751693B2b66D2FAFF535a31b4730573": InitValueInCell,
	"0xdf131199ac450112A530d64D737Ce18eC5FF492c": InitValueInCell,
	"0x75873231220dB8D6aCC260aB9354a22F422c4eE9": InitValueInCell,
	"0x02c562ff34F84c8869DD9AA719b3747cC26bF44c": InitValueInCell,
	"0x1E9d234300611F406af5859d030cB70f0d95370b": InitValueInCell,
	"0x3C023E632A7a727f8184ca3f0A979D8ecd2a8f8e": InitValueInCell,
	"0x1EfEf012192fe2193bCC279907f50B4bB2ab110B": InitValueInCell,
	"0xd12739cDb159E31A1fAe0742493dCAD6Fd031d13": InitValueInCell,
	"0xDeBcB4e000F54B9fF9A81322d744718f83b4ce76": InitValueInCell,
}

//  GenesisAddrKeys maps genesis account addresses to private keys.
var GenesisAddrKeys = map[string]string{
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": "77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": "98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": "32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": "68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": "049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": "9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
	"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": "b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
	"0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4": "0cf7ae0332a891044659ace49a0732fa07c2872b4aef479945501f385a23e689",
	"0x8C10639F908FED884a04C5A49A2735AB726DDaB4": "9813a1dffe303131d1fe80b6fe872206267abd8ff84a52c907b0d32df582b1eb",
	"0x2BB7316884C7568F2C6A6aDf2908667C0d241A66": "4561f7d91a4f95ef0a72550fa423febaad3594f91611f9a2b10a7af4d3deb9ed",
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d": "0x36BE7365e6037bD0FDa455DC4d197B07A2002547",
}

//  GenesisAddrKeys maps genesis account addresses to private keys.
var GenesisAddrKeys1 = map[string]string{
	// TODO(kiendn): These addresses are same of node address. Change to another set.
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": "8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": "77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": "98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": "32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": "68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": "049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": "9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": "ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
	"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": "b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
	"0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4": "0cf7ae0332a891044659ace49a0732fa07c2872b4aef479945501f385a23e689",

	"0x757906bA5023B92e980F61cA9427BFC15f810B6f": "330fcc18cc5c9d438744ad9b0f4274d5e9d34f099b8acf0066e304f4acabdc90",
	"0xf80927B05dc9a25247F3d68B8192cD78361C79f0": "b0de1f5d622fcea073a06386ddffce526165b38a889c04e5088ef88a3d807570",
	"0xA58Cc74A805adb80DA35925F28f7B732Bbf3C9FF": "209f9f8182be0f07f785be76a35c82c7ad2be4359f5c95c610e005dd23e1cfaa",
	"0x9afD6D161E3e19c0191d83D3f48384DdBF24Ad7b": "3b33cfc81ebdbdf699d6f9891f8e4f1f66a8e61372416a4ed068985f7bf2cb8d",
	"0x8A618C045Cb00E8abf6A9cab60edd10dd5572119": "99b860be69e7015b0b6d14b45b3d8534ae8a7b58c673c81369d6d61de511387c",
	"0xA6E9bA68b1C3d9b45c1a07Aeb04169465CCAa69f": "f19d51628268e254db4d7f3a1108a72c36073fe4b4298882cb729b690291ad69",
	"0xAdea9867E143445A92A5DDD11B447de59f0090A6": "761225588f7180bf37fa6d1e2829db3cc4a8972ba6358f151c5249bffa402d1e",
	"0x5360D2e4596CB10211bef5e088A3ED705a6872B1": "5c0eb44a8d1219bd995cfd07245ba3bd6b63a9cc76549e91b0641cfb888d092e",
	"0x74474e320Bf05D9D83B61946faC7C0c49C86e634": "54fd01571a9416fd213c62b8fe62b870e8f1cab5f609ce39e51daa6fdfac2535",
	"0x3c60F22441283FDB7acdE17f1758673e7024a68A": "dee4955aa6bc6d74f910ffd586f2ab16ed8c63b580f49fc6ca219015e0a80ec4",
	"0x8c365De6Cf9644276Ce3410D0BBBB8dE06d30633": "229756158eb85c278fd1c5c909f0fbb0466cec43b65713a036ee2a8feee51360",
	"0x04b7361c47432eA35B4696FA9D93F99fC7C7FeCa": "0155b454947fc2647bf1d500a63e7d57119de8b428cc8f6a0ad329963f8dd36f",
	"0x26fBc0553bF92A1f53b216bBdc9DF93F19209773": "05c82e623edde71f3613a0d6f2885caa00a9b03c10f62a82d2c8753ff8abd380",
	"0xB8258F7EA7B4C0a2Aa13a2F4C092e4a3eCf2a379": "bf0abaef91881b51c4d9141893e9ff8f8782b6155415c6c1c0ba1c83b5e88207",
	"0x1c9bD2E569d990c8183D685C58CFD1af948D2A8a": "3c70d08ea634e7f1e511534ccc94559bf069abe1558c6cff4aefd3413cf725b4",
	"0x90056191A8aEE8c529756bB97DAa0e7524c4b1aA": "8f075735a79b61d1cdcbd24dccf9701e433814825fe5853f7b6ed21884ad306c",
	"0x58Aa25D60FAa22BD29EB986a79931d33A2B9C462": "20d11078782250c7a65ba7d3d304ce703a17322b4279a89463f1c96589b130df",
	"0x6F55C53102e4493CAd5620Ee4ad38a28ed65997C": "0d282ec335ce58f73d69b633a1c0b31fae25edc66b7c755110b9a94575e7f811",
	"0xA3a7523F5183788e1048E24FAb285439a92c3647": "8fe30f1c900334741692ee8f41669d6a8acfe5a29e0f788652f236552bd22c3a",
	"0x96d359D752611255eAd3465Ae4E310b47a5a20b0": "14d86d15da9376f8b0b75b340e213ffd984081d77ecb6d3365dce3a25a3dbb7c",
	"0x737E2d1562b16FA1Bf8C7E510F0be32c0BDA059c": "375eb77a7dcd5ccd6becf6c1ae4a07ae4a2a36d55b52231afd8cd8839085b452",
	"0x291bCb8f5199Bdcd8c611b91209f1a2359AC2FdD": "93c5619c33e0184cb6782551421bb210bc102e185578c687905db0fbd23ee472",
	"0x595e1c030E76eF64633046A7Ca4deCcfce952C73": "7e91210a633c6c59b3e3ee09cb8e13345e14d8723bd6959e7907da2efecb3957",
}

var GenesisAddrKeys2 = map[string]string{
	"0xF344eee5F1F689735A200244648934FEA73b6af9": "3ef11c8716a7eabac7e154a981b6c4ad23cf3c67309495e4c2b5a817d478f4bf",
	"0x5613033C882124fAc94004E84D684504c0474C4C": "1a302782d1b21a2f7d38e560050a6a968484daf9082fad6ca5f8b0804780ba87",
	"0x476c9e55797aB3Fe5700DD3efDc5d62333d5acc7": "3fb280735bd815037f3a3a7b797196b1d19bddccbbf1d58c423e1e8315c3bb8b",
	"0x00f798863bdBF323d61C82Cf5Bbc564006e90D3E": "2db59e73cd44b0d6250fa592414ad701ea17a99f67c52620ccb49a9c885e6fe9",
	"0xc4Bc10Dea9CEA29ACe42A52486390A60eCD15E17": "789a95463b57b2bc8205e8c027e054eed94a3c191e867f68ea2eda8d3b09cd1b",
	"0xb116638e938023fD61D81D9E1EA38230Be1c23B9": "31fb0b948253a64c99eacbf130ae5d810c99a82016f94e8ecbfd42d7db3ed16c",
	"0xeB431cE9b1Aa25d5AFfdb4850B2A54666355eB68": "dd7aa097dcfb58b6ba8bd2f7cbe8f8eec6f4cb55d8d391a9299501fbc9492788",
	"0x9cD2CFa83e6d85625a64fe31779446944c024E36": "c07bed6441a92d0c8b99da1c4a63a2ff1c6e233729e3bf9428eeee25b8b60996",
	"0x19F2A60e1819F8C5b1fB400d16c7981AE6d7bABe": "104e2ab1eef39a264481bcbd2415ff7debcfc795e8b6427a2842a395f9ffc436",
	"0xfF89F37182d98c737956eEBFD78bC43A0d43ea9d": "c8912b90458bbe08662c57227d2d7663400e3ddfe79cc82678fd9adf59c11497",
	"0x1D20e9405488673D82EA7fAB6fB1267eF371a8e1": "786aed34cc3533313454280c3623f4383e189052a0bb0d988f74d870f440385a",
	"0x2CffCDB04c30Bc31D52032D4e797A3c7664cf94F": "14bad5a8aa887f1e9c0338a3bad0f77c29dbe6b39d6770dbed3ca102787ca8",
	"0xD5AEF43D86ec775305Ef7A6C16487b2a8b63B97F": "86e746f458294e8450e3a0af54dbbc650d3ae2049548f4a34e90608f3d7fbfd5",
	"0x62B039889D27f20FAd29e87C4d68EA5AeFcD8d37": "9b76720bc76672496be40ed683fc1388368dd1a0927f7baf0adc07e3e6a69793",
	"0x53a285e30D48a6CA4494dD19457E37B4fBC078Ef": "f283e9ba42b712e416b209c282359d3552bdb61dbdf942a28de8881bbd1564ff",
	"0x3E73Dd17D36398cf047C47178248c6ba820249F2": "ecfb665b5d44115e9d86a62a2d15ec805b29e0dd3a1cdaea3e4e7e593491ac28",
	"0x06C8a5154466a17D53c40379A9ae06C4ea23264F": "191f579742da798cab1796babdc63c5d3333b1fe2d531fe8d8bd44433958b38d",
	"0x8047Ea29836cB6181241Fc1fFaF59C31e2dC5eb4": "36484a9e232ea547cfbb6c3186e1fd76654b9ca4c44fd0bc7beb5b3b9794a00b",
	"0xf6237F9c2565069B8C7Fbe35f7330B3BbE8135B0": "5da554d2e8f80bf6bda0648095be4eff6f1e425df97f556970361caec5ec706c",
	"0xAA8159b4759f36eD00E08A62aaf84D17afFeCa69": "3334fbdb46813a683048b705ba6fc98c0d8c80ffeeb5c3b7915e20eebab62717",
	"0x6B685293d2Ea983878781F7daAE1CAC0589ED308": "e4b8a8138559c57dbeb3d1af33611ac11e783eca262dc2abb53d977abab0d321",
	"0x0e1665C86cc70C77be465a99Be5398746E3a04F7": "350e2b8944ecbda97e8b1cc7686673c24250cc64477d2e9280a7c95df28319d7",
	"0xABDdf28B64C1Fe43753674fB62173212EA88c50F": "0250f9ff65149ec53096aa0873a66babe71588d60ff4b1b1bffd260119aeb9b9",
	"0x309674CeA993D5D35C7155AEBD302cA0ABc6e1A3": "78be783701c586fb0481059bcd66c143d659e812b8c74b08655f4efabf5c28b2",
	"0xd97FF0890162ac5044393a92C6bA2F408d58FB1c": "7d4e128bacce22a559c51ad55de8e0752875ee8fe01e202b2f94f4dd614d66c7",
	"0x975DaAf256C4E9cdc98D06FbdaBc383b7Dd2B00A": "6bfb9284311e7772961edf5efbeb1fb27443ad1b61b6b22d38ac0537a15c00ef",
	"0xF3e4156937328f284426370BB4956f7ace172334": "993ea7be01ad1845f91e338f072bf6fa23af260748f219e2515b039dfa2e4ce3",
	"0x13744BA2149999C15d1b4Fe0Ae19ed53698703d9": "9bbf778cf5098ff1bab2c6d24c7f2c5cb11de57e5328c4061f5816671c3e7471",
	"0xDf987D8d4cDBf182D154401141aBAF0c17DCDdb1": "74b5edac4eb572c0f4bfebbc131d8230726eb6fe1feed7be9aa90f5c6e09f663",
	"0xA4B5417B7DaE65AAa81cF5a3109b44d12e04522D": "917e0a010b770869aa9198a3829ce9afd1cd12f415501ff3b19f4b3e421b9301",
	"0xa7B6dF9306cE0Fd7885596a0e9b5C637C47Ab082": "ad885eecbe2837909eb2ce5da984a9ec779cb9ed8afcc34985b18c31c1f69fb0",
	"0x5cA86DECF53a7Fb4841cF58787F87C1743776F2B": "2121fec736816fd78f5925322237be2eaadd34a8cc0c354ad8af341fd5d0a7f9",
	"0x9d6546D4fa4E9F4167BeF19818aDac7933928405": "8412eff2bf762196094706df5cc488695139fb2950e7036eea45778f42b1d6dd",
}

var GenesisAddrKeys3 = map[string]string{
	"0xCFFb16DCC027318B141a70c94107a9bE46bf2235": "9c1d9e72c60b63db2f8208e41d1ea488624297e09fd5ffa6b5eb34dffe36b8b9",
	"0x2e8f56089C3a257CAAA475dbD11bC31d85747A40": "3a7c354f2739d18d3a94029851e2c82b7c61ce47c7de60189c0f437d71221a5a",
	"0xE75E147b47705D262Dd1E936635C7B05567670A3": "a82fc68da29a87f1ed8fc5deb3e9b86cabde97fef06e3e8625f3d8606f688100",
	"0xBAFB52b89d57ae5D843ABDDE286569288070e94C": "1529551e2f090416c7aa5463d64e0e3f446fd90f27a739764f4717b5674b9416",
	"0xc60182805A7299FF85DeB3f29b6927e05B8082B7": "7f9efa85c09bdf937e8a08121323f08d4d402e957c22c00a69f79b774f65385a",
	"0x3aE2e9D738574EF83F09e2353aEd698251D1a014": "62979fb82f6abd53a26eee17f375fadbff2a924a378bf190bc8e95bc7d2b3f51",
	"0x2856532582895BAceb005FA24F011CAd5280134F": "2c6458b90676f0bfa261c46e5c60fb27a87bb2b06e8069244c4a226a5e6c850a",
	"0xF00CB2F0d7A40724F68f943f8c4f4a1Fe7cE06B8": "8ef30f0b830b74dce4aff27ea86262e13c4e06bf8d6cfc171a78f9814f05134e",
	"0xD2273F595844b66C128178E2245bef523C934986": "ce299071fdf6213bc23ea35519a05295e6f1ef242f2b5087605bbadce36c24bd",
	"0x8386b8aD0Fe0b774B4e34B3506A438f232851405": "6447dec422443a8397364568f01704074bd15721321c673c87fe57a49db34498",
	"0xB0337D3eb5B6c0A872C5978c4DD6163B5c392146": "58c70647d5dc0eb0054401a57224216fbb99ce8bf4ae4ef5f91d94cbe3c95539",
	"0xACba7B9eF6A4A9d055af4DD5650f2Df667F1D41B": "34ef2902995c2aed8ec7b3eb2b30ea1839f7da6e73f565e0822a4022468436ad",
	"0xfF2cAFe6c386Bd383fE791edC2ffdA8383E4A21e": "677a8326b6e002847aa49a162649e50a92266a2d135e8c1de615c43d47ec99cc",
	"0x09CFfad4f632353a8B26C014129808a6F9cdb272": "225753136c13d3b86c9f49699dfc9c10f9edfa7f4f4e22559a77a41d82cefc5b",
	"0x98B0106889E77cd28B12f6e359b0cDDc7CAcc542": "fee4919c5b01f8a2b6c506073363e096af3754de737c5f1181da35101269f836",
	"0xaDf56017d13258500EA8C7f67e5957A8E8Cca6fd": "25dc64c31c20b92d725436ee9deaaa90c51e2a712b93ab487cbd14b4c073708b",
	"0x7d4b7cFA300eCfA99a4EA8d811cCF40C271807a8": "d6171c8f7fc447a97f6cfebb02f567cafbffef56ff1ac5145af91622060a7847",
	"0x8A93d2Aa374187a34BC03C679116d046599b6D3D": "8d8bc8e7e01032cd2672ca7bdd9c89364fa397404dbbf0cc271fa9d89414c4df",
	"0x24bdaA15F8D8C7889d2c66f6b8eb116eb084baf9": "16505492189509c901ee93275870bbe13ada18ea7fb1a5c782c549b122677ead",
	"0xF038bE927215E63DfE371894e818Ec96d2E51Ed3": "bc37ef746f247c32fc96ab4ec70ea218aab45628191f11b36227bda4e870ae95",
	"0x74036b7Fd50f9D4a257c0cED896Be48aFac9b535": "fbea9d60d0baf4ac7e9971a48560e124cf820003339f545b29f528fa3dab7c62",
	"0x17d99DA7ec2a181677Ca2114A09Bff20C73EDb5f": "c5c6d76a7c66a8c542c9a915e2f9c2b010f8dc0423cb029a41b0c8130e1a113a",
	"0x2214F1028Ae6C816dbB772d1333B7Cc801075a6c": "bf21d8f9b61ec251c196aa549825d07147ce753eafaad48b5f42b53f718ef12d",
	"0x4Cb633Ee13B0f1Adeb5B34B2856B3e94F31619c5": "237f116ab6b3f8bf1ae4a4eb01229402b471207ea440a2e67c04959bf4c0d374",
	"0x49E306Ef7d6bC201F54fdB4307AC79F01BfE5623": "481f3851c8fc56061661d8427b8d493f29eee3bb5abe908fcc79b36c6bff9209",
	"0xF3b30813B73e35fe2068a3eFd6C3207e5f8b190d": "4ff2eec5e91531dd35babad6ec0a319bbdb7804d58a55760496b52653378f5d5",
	"0x015803741Ec982b7bC8Ee658A98eB044d5c948C7": "05d720c07a7431eac8afeebfa5e0e0102ca2bcb7d283e673de21666be78a7643",
	"0xf8b4FA2558aB6A881813f911Ba09E41885a87E6a": "cd79130d34acdb7a17cf36e902639be2ca69d3300459bbc25a1f723bb4d43a09",
	"0x99eeF578CAB154051Cc57F35945d1CC805c6f94A": "55e679fbe88552cd660f55fb5fd84bb14107b565a5607e81b5f8c981a1087dac",
	"0x5af9Fa9DBB707A4Bb0d90A37F8b09E03a855d0a7": "f43933976181c1ab6cb6fed88ff00fbd4e7ff1bc7ead3ed957e23150de12b882",
	"0x6a61e9Ff463AC20520dd8cC41B9D87C7CE562200": "34dd13c74d7f45ab9855e160dc74daa2bdf028772d34cf04558b91e3fb1a9b95",
	"0x9F71cf60066d5124ef311a78acc6b25aD5098f06": "f1f884226ad0463cf32dacd2fe741e338f660d1aa93ed34a7c962ed8c6713055",
	"0xB65a35CD011447e5ec6FA547a44Bc8220Ad096f0": "a178c328bc93d4d15f77e88db214a25afdbff65432a4da70d4f7ea69cf804f74",
}

var GenesisAddrKeys4 = map[string]string{
	"0x3912f280C8d9bb0656ce7cA2412E15F2872e9390": "9d5acdde39fdde38a33a753428988d390506f408c4885bb5c184b3d721fe548e",
	"0xC30eAa67ECB6D14617c79C0C7aF1ecF94B35c1b5": "b5ad291d0ad6fc868617cdd555aacd91ba36a825e4d4e504c79fd53ab017039e",
	"0x454494bA1551375091cF9Df63FfC6C3e80D3f245": "d3c507a28b6ab3ca98bd59652f2907ab18a5a292792e88109a3972c61b45760",
	"0x56C466293ce0182a1Cbef02C40929A54e489Eba6": "a553558d03d82972855f3e0e3517eac80417f7f7aed8714526f87e1c7e8b4de9",
	"0x93A67DEB58aD74aCcdE34C724c26901A21DAaE5b": "c1b4e4fe862ce3f03a66fdb6ded45286f735dbcab3008f85b7ce75341ee35ce8",
	"0x209fFe041aD149D62AA22F0e7e609F56d7C8299F": "baef9480f75cb734edb4247fe9682ecc67260e95e9f1da7de7f273bf25d84c41",
	"0x2212b13Ed0DAADfB79B7b86ed29ABbfDA0b59F12": "eefcc1948609ceb82d86f3f32f3ae7f168e6df593d53dbdff7e4d7ef2ca24d16",
	"0x8870c76ffD3763f106f3cb1cB7d13bae0d7Ecf88": "34a2cc417570a26df89bbd82ee09df8be558c47573801f2b9edec079e6ff264b",
	"0xaFa3384beBb80F8EAB0dB6fe9E87Cd5ae7496c9c": "bed68fba992f55e39eb6f44a90582aaf67f330ea917a71badc78dda2c7cfd4f7",
	"0x8CE8708075edd96570ac68AFb60196B1bbaECDD5": "628564c80635f9b1f71ca2ac3fe34b9cd44b3bff285b93b9a1ff0ae880febdf4",
	"0x23f7e9a64B26787c06CA68B2a258f759E4F9a4d4": "c18627c94320ff16c9f6bf50aac491e58d24a3691ce96305af9b8f069976516d",
	"0x275Ce4Afad60B0D3D4AC75656398107C2e916E00": "e1a2cbf33e77145a4f201cdf8429450cdd038b942bfc44a8f3bf0cec6973456e",
	"0xb95fFAa9281eB45e43106De70B66E4134A5DeD75": "f8b944d7e84bf4a64b2f06c20f97d5dc33a1c364726771be98ecaa6ba032d670",
	"0x545f17337980A46953F571f5EbF6B71994411DD8": "41bb627e629a2fd6f5f75d33dd8fdde81fb8f143686783f89a0501e5294bb5b",
	"0x55541DFC9F554275CFDcC83D2fd94Ccaf6fF602d": "defdfc2128acea8062714d20dbef1dda745d683d7afb02e3ce449408acdc3a4a",
	"0x21A8EB4eAC0b2dD589EAE47FDaD182EB6D2d7dd3": "ecc6b0322a069c197d3f0458fb0b7b2886d08347fa90826215553a145b028207",
	"0xa894CFf793F53C76853b7a4e3Ec97F4E4f17FE51": "e7e9e4a452f680e331bebbe91caa7d0b84a3d47fdcf6f1bdbc294c4b1e88d666",
	"0x75E8c0D6d368462AB93c775A1Fa960fee3531ddB": "5a5f9beb64a2cc39b524a243b981b3e62d157f4f44abd310869ec2636f36eb84",
	"0xa109138bD5A7c77081DF75a105e4B11e89BE882e": "d6319d35cb01fc5b87b054e696d9c873dff813769aa61a1813614942e9f47ba",
	"0x0512d6cE79D5922c048caAAb814e9480e78C10a6": "5351aa1ad8a201a4886e4994a14d7e79080df73da0f6117ddc09233e0e97b671",
	"0x8511D1c67427fa6199Fd462D00eE4C1658c03010": "c644a78a5a879b6a2ebe63c00aa7b84450ed9dbe51e2235f5d3fb05d9af05ed6",
	"0x12454E679130F4935Ea240Dc53ee8a9D851f19a4": "cb3667d5c05e00cea03fd99d46a9ba9fd483bcb532f18597f82e418e30e4c9b",
	"0x6C83faBA9ff8Bd36D147f158795A0D575047B361": "bb3036aa8a970fa1e6fc823862aa7b81e0a53aac9691fe53951b6cd34d8c54e6",
	"0x26F8c1bA1B3259c0479a95F759A245dAf6249127": "7aab987d8110c45d8c4a1e947ee5eeb81ba9cc9a9429da18347dc3927a6259f6",
	"0xD0bfD16E2bc10c9a8D41F9931A633fe66Db4FAE8": "78b5760368e9185be2f5047354dc1df34b680f1df0074e2c0f93d250e258f87b",
	"0x652418D8eB4bdc915b378166692f6a3C9f66E394": "f03bb1fd7084f4b2a8bf99ea383fb02ae5d0071dff05798bfe8cd70e669ef7dd",
	"0x7D947416382828560b6db6fBE3596EEBe3C60c80": "658883bbfeae7aab44f053e79404b1912dfacbb830b6536f0595c8d117b5d327",
	"0x233184Ae533849D7C61A2Ef78683ACAd30B918c5": "75230feef8a308f220c2fa1b699a47fd25a20adefb1bf66ca615cb5955838511",
	"0x175403013D2E77BB91Feca5CBeCbCe5E5A919920": "efc40b309957cc78405834e5d4f32cc599d9363943195c339a4f2e5d5a4115a6",
	"0xeE1A38f9F72811ceB5D3397354E9BE1eCDC1aAEB": "7b49d6406975b49e26a4f1884dbee9caedac6f56a52cc78ca893fbb20cc943cf",
	"0x8C8Ff7121A4d3858Ab7AC4ce2FCb144ACE90a376": "1d6db9b21a133cecb2dec52f97287257d7bc6f70be66c2203a3d63df412a604b",
	"0x945FC459d3796035074Dd8271d9257029a19c610": "ccb2648654eab418ca7d2ffe1ed587c8ff26d6989233adbcfec04d9d1bd7c6d8",
	"0x2D69f9D718D76849850B4C892b5c34991f9644CE": "c1825c1b26cacd39e4fae4726a9be52a273d2048509797b38c391d264b6a9a6e",
	"0xAB6b817dB3F42089b5338D2b813f83208D8bD095": "94fd2165204d47d8131341ca6aab1843c32da7015430b8b6e2855f318341af6e",
	"0x248c509a937400b9811Cd5952eb458A713960C8a": "4a94db69306f043e6f87ff21078b13fc1bfbb0494aa19df31983aa2937ae01d3",
	"0x496Ff51362f0f2ed218caf95FAC30e582a958b5d": "15b0cadb84cddeb5cbc7d3742caa60c9301ad2bcaeb4928d33aba9f5942dba17",
	"0xe69Fe5D10CeEe236c72F8f73c08A174e8BDA04da": "c37dbf8dd00468219f9fb222b93cd90fa24e9cb39273e69bd20168c5b5f8a83c",
	"0xef9D8Cb340A0Dbb1F5ffDE9aAd882B0ab83D2778": "4e986e8a3aee4c8fc5b32f4cd01fa76a8720d37a8c0dc22c261959eb2be92352",
	"0x5f08b4aba7e1e1B5C284475b5c56A3eF030f3465": "fd9e5ceb4fd23386dc2b27ccc9ab08f9fbe2d02f19ae692667408b9d2d53162",
	"0xC74887eAcaEef30378E1842C55Ddc8A714FAd37b": "3fb0c0a833cfb29bb311b3213dae659a9960507988062b92c61b95cfebc785c5",
	"0x184D41132c84f67c85Cc355058EaC90188B80287": "fbfbd32f06a3a85c12a1e9fb8aab0443c1248bba00ee27442c57d9ced2c3f5ff",
	"0x2C0369DF113f65650E50CC5f96C74a0cDba567a4": "b8b48c2faebb02a849b37329aad6ba2634f315c7587bd8c55528750b357c8ef7",
	"0x39474CcE55f9d10005AE10BB58d86A5a9332527C": "53fce956b3f46d99a054ee4b57a761195c1f2aff133c786614aa8aab22d9ee20",
	"0x2E3c2A38BaB95766bfEE3f6ABE9C306cCA69EE8f": "f280c77262db467fe33ea6ea5078bb3871929afc038edf3d41c4aa1289c2e236",
	"0xD29b82C845007ac4Ea871FdDf0BC89F7BaaDFF66": "81b2151bfa6ac03936c88624bfaae760ae59deb1e448f7fc05bf82b61f040d81",
	"0xbf45D3D5894080BAe939261ec5ECE383d1B86933": "d52e969fe3972369aec689574af1d30a61575326c37f4c4d9837dc83349000bb",
	"0x4F1274d4697D0457d91Cb607f5B6981A2528D79D": "984c8811ad5d25a023f6d99715be137786ecba850014267c2bc0120574f9a663",
	"0x21CA0f6EDbeB87c22B180233E0D7DE5a011c49B9": "11ad94d1b0d1b0ccbc539ac0e8f6bbcb10433af7128125a0d2acf910843bd97c",
	"0xc2E76121CFdaB0ea45831d5E9EE060B491B94248": "ee0b636e0596558a85b94efbe4fe2dbeab93756ab118defcdb5a57e63a94cb18",
	"0x8A119F3D82d4fc6872FF8bA1dea5DCd4beA229D0": "26b6178c48beb762e8d974b786a2c36c4374277d63eb292fbfc9f8ef87a9e22e",
}

var GenesisAddrKeys5 = map[string]string{
	"0xDCffa0C83180a0DE0E374F02998D80fF72d03b8D": "4924fd8dd57e142b1a2d15abf167d844298af8ad1d9fa60a23fe5eb32a8bf3c",
	"0x45F14EB98c2AaC73adE03c2172F4A97527888dCD": "cbfe8436f132fcb7dc9b74133fe137edfdb45722ae64026809e24a5abbff32f",
	"0xFFa98aa4E68F8784415762615708Bf08DA72BFa9": "fc14bb527ecf08ba132a40fa62add2ad3f60234ab4494a6802b692d55feed3e5",
	"0x7092588cf39599464C8DA2bf917e671572CDcc2C": "9d974c26ec923817a8e8cbfbea51aba2f07a9666e06934b3ac5920ae7f6b1c27",
	"0xe1a6A1d44300159Daa6255d411e5F90526b68369": "c202cdbd87c4639df8468337cc9e0dc239ef36d0de563d1e0b6ebf0e2931bf42",
	"0xec260c042cf10062eCdaAf4D7ce071c34D09F0b4": "323d19abc0096188644b0e0d228ec79a7b7b4536f5a846962085250ad00c738e",
	"0xC4D5595Cb104e423235380D485154eAa2E6605Be": "6e4df6818222965fd56750c36f81863b2697f6202a1d806faefd42bd2d6e048e",
	"0xa751E49F4eed8609E9014fD240A2e9A5FF52434e": "4ba140a8ba534fc47d4f327818dc8ff7c5013f3cec541eb7a658aed417e0bb16",
	"0xC41824658A40cf5a79785b30dFf0F0f72eAf44c7": "7a6c16d1ecd370b892c135ea6f595d57cb206b463e11b5f9616e602af03f021c",
	"0x3eA8E2c5067b6684a5e354445E79De0Ea6d3cBA9": "8d198e8f5a01be84fd8450b22a09859cdf1df0369a6faff5eaf98c565fca08c2",
	"0xb97cc403e5b8c4C2BD1229aF56875971df9aDBeC": "b6dcd5180d7fb937a0ab4c4b8e6686a019a80f4912bf442c7acb774f43112786",
	"0x95896fFc832f64c81679cb2Db5ECdE1b21C571b9": "4d174eeab9e24f3546503abc1ae930b718f71b2ee34b5d189b2eaa987a6245dc",
	"0x29927eE1765bF021dF27aCc3C7654d84ac27b177": "f088d2be3323b9fb503857150230fb081def5c45393846ae888794c11ed0ddab",
	"0xf3A6bE146E96bf668Aa7Ad86a00E94bBdC1e3E36": "68a38f1ce6a36c4c9e13254a691f63e4444d55c0b9b3fff0b6273f4042e88942",
	"0xc0d43531bBa86cD8041C8ac995646A1443946D11": "ffd2000fdb5df628da9d81d426d8303b3674cad6591c149e51600e111452ab80",
	"0xbB06BF3E8f6e866d9d192663083704321CeE5a48": "a70904b31754670c796e8fb84d2a303302bcadc42d08d4b8baf95301c7f58739",
	"0x08C67737da87b7A58308A5354fCF9175a6bFcc0F": "7685919b85d703cd025b9041a08418bead890b53a02c4cb1ed198d1be3031571",
	"0x9147c6e631b78B9C7623A847E3392589938F22bA": "95bbd2a825bfb29d4297c044148dc98c4924f4e7d6af353059185031f4871dca",
	"0xeA8D832A14179bDD6884eE0481804F31E9E46D50": "e26b7e8cb9154cb49bffa89fc13d005eee7c16f66073dc32adb721d69e298b4e",
	"0x10ecc59b7E06D13bE3fc30855F771Fa68d826F76": "c4849a9fef927b002abcd178f443a26f6b46a0a44c9659e3cf7b9d1d92457ebc",
	"0x0bE089Ab406B7becf34b3C5B5b64d26702ED57E8": "24f226f175d2cce39ba9f8c82c4e015e1bf8f23f3828bec211977cc04c906bf",
	"0x41c415262667496bE6B86329aB70258EBf4b88Fd": "d23a8bd41d48b6ebdfcce85a49b8d94476d43dcef7df8deacdcda7891cd80e23",
	"0xEEFbD5eD52481e88e358E08E4b3F908174eC881b": "d377b17f0d92820d5b4987c2857ab3fa48aebdc4c11377ad3cb2708c42eca776",
	"0x49722356FB53D7Cc7cc713C919D538A3B4818B87": "505bc37418fd5d48a414ebef72acfbb51f85a58097c2696df170011e2e8d679f",
	"0x0469F79D61Fcf88e7a5a6fC15b466f69f39CbF47": "eeafd1e937dc3418f196c6b5a13dd5f8d3f84322fb3d17ab69866ae783164ed1",
	"0x28d18554f1621cE2fad7aa5cAC512ee11A8b766F": "b471ed756f5d54b737e3e1cdb6640eb9302a587552cbceb7a3b1d2f99a66b354",
	"0xa624B51e365dcA3f1B96F741A853a282741c4f0F": "1dcc20333b4ffef0663ea3a6d221dce2cf6bc108daa97fa89fa66df04e9ff399",
	"0x9ca0BA8d38eB8E0b7D4ec0dDBcD5E9d5fAdCAbD6": "6b3fb33d1c5bae39b28586e7c3021c06f798f19e57171a7d43ee095acddfdac0",
	"0xf39d6c14F159504B20A8868Aa3572d458A989F90": "eab6f62bf008fe34fdfee5b1d339bed79b9770e216c3f71cee41d9e9bd8bc153",
	"0xB420ED62853396ff3B67332D77b96D3D73B74355": "5cdd549a7970dff7681b5058677c1d94845590f485be2ddca197bbb3bb121d18",
	"0x465D3b00cbb61b7901021179Ee6Ea684B6CdadF9": "e13b7f85bb4d20ec935ebce8e32ca18881d63d18e77cb1198892ea302bee7910",
	"0x162eFcC902a8Cf55C902a9e2a0CAe0D7B0B8E463": "9b2859bd193f6f4a6256955c8035167b22763604edc205ba62ae950e54426239",
	"0xD688667B62B4D5251095A8Fc773e3847A6FC38a7": "3e776af6778f3fb1fcf790ddda18c832cfc3a20511c8f9008a9721b39888096b",
	"0x9eA15621CE809181E93815075ACeeb9FE1e106e7": "39f1d5a8781a504d5c524c1699f47e0f0ec0d0bc91cd8d766f55d6ec6f04ad9a",
	"0x78bF37Acf2360E8ed6bAaa384202779EA0df472b": "204c1913e3e0b4e6bd6945eb4a60a9446f4aa6f6582280071a46c20fcd6e80ef",
	"0xBFeDC74482D2a522C0b45E798E32065baf28ee2B": "915f289662d39283468c1e5b5345947f5a55fd2e76e1cebbe00419aef96fd297",
	"0x2d34F30b80687290bB7077bfCDcF1250e067D429": "237b722690263dff934154dd2e41807d247e439cb1047b94ab8bc1b84a69243d",
	"0xBA4B14E4D9a82409369443A76BE4C5dCCC517eB7": "eff7f8f58f8f28fc0ff3b81896829adbbe7256b26dc5bb41f193f1904212e348",
	"0xf73657DaA3DBdfEc68EB18fB57fDc323C26598b2": "c6ea19a9746f58225ee702513ae6a3b8941fa8b55d90c9b111d37a2d230eaf80",
	"0xEa4D2A1a8358156C1e2B6a58Ebf4EaF270CA6ce9": "c221a1296fb8bb3d8198e1dbd08641b26608fc44109593d4f52365217ce36b1b",
	"0x8439705461C0E84f05dB56B3F746b67056a6dbC4": "264679eebc40ad3e5dd917839b18335fc813ffb5ea4bff0adcc9e93217180530",
	"0xb3bafdB3020070A3e08D43eA57bD185f91083ab9": "8d3231bb423f44a265db9bf8041a436af6181048a511dd5f50a9653456fc8f18",
	"0x13603d5420224A10A5818fa423aB21C0fFEFE869": "2156d74ce73188b6ec194b4bb5c598cf945b01b9d0d8c12123e5cab5a4e7b3b2",
	"0x9ceD9a019F054476b97c5044D48C4480F26abDAB": "39ac50edcd7aa5e8f9086ffae53a77c61fd2898af46a87f5e19b7db2e3c0a500",
	"0x5681099b18D430830d6c68049375D5c30F981Fda": "4f24b16c54a00ae961bc7633f39fc6deab1977f812cf14eff89f25c1fe7b3534",
	"0xf2fb843396f971b8299619e56440B82495776a0F": "ef2f08ff129217de1d8d4f64f6dbfc44560d0acf863bce13fabf0776f93a4e61",
	"0xDFE8FFcfB693A4AB9ff10862b4Cf216Ac077129F": "79c7b5c75440ce00081569922039f920884f5e62c08c318e9c9cb9130243a553",
	"0x4139A7DE73C1E9C04F873C5311be5ebA7b51a10b": "2a0213fe1c91ffb2493d6de5b651934d560e8a185eae7a880e4c666d44e93b9e",
	"0x1c019eAB8C44F18bF32EE3f1Ee32b5eD93E8Fa36": "4443e7880bece1d2e830041376bca277ee373d725c13748f9a379ca70b5d6b55",
	"0xbEdd2694361ec0EDb5ac99E95f44EDe578A4Bd89": "3d37985a223536d4e1537a35bfd06fa842b64f8ad95718f6dd7507a2d8d9dd52",
	"0xD39af14644165573dda5a642DfE215442C559F65": "c74e048e52a0ef5d443962c08b4b7864feda917b42ad832e2db5da44bcbb83aa",
}

var GenesisAddrKeys6 = map[string]string{
	"0xf382b626c0ee59d9A281537c121D307C7E524C00": "ec13b84d932e5312c499933725ddaa8a4910ed36d0a8a3e486cd8f0683a685f5",
	"0xf967365a67d8D4107bA1b58bE7136bf9C5DAFa6c": "873fdf37a79d7b0ccce8b6c72706c35a7cd298e2cc1f774abd8431bb1a500086",
	"0xAaB6A9b24DD05d61E621590326B53443955B6437": "d6bc7b9d3a704cc33d6e9331b5abaea347a71bffebde85bc54c9ecf58e7e4b99",
	"0x3f63Ec11e9d6A4a1005fA86139831185E1a0Ffa2": "59909b37f7e5c803c8fecac0403f9f0f58394c0b0edfe0e0f16d52d01d1ce337",
	"0xCc7Add9B29091A37f9513628A15f686D7cdFD4b1": "d551a473ab3a5203dcc925b5b874dab9fc8114edfde599d696b0d5da100f077",
	"0xB535c49DA28a3B3B0AdF7cc49420fDD660EB7fc4": "f8a0fb599d56d38d6ccc63d92d490773afe5aea86ce4d2e8bca3c286f9ba882c",
	"0x8b1a96aa5F29d076baA07bc343C79BDc026703F3": "8e8af8cbe93dbae171787c9d954faf931c6d634856a7e91c2953f322483c9446",
	"0x5900C620De82FAfF0a1bC483b92cA18AAfE6C344": "5f5ee1afc02ed75a5e799921ce3d797ad20a131cbf2319dcde590147d748cacf",
	"0xA86bFdc8B7025Fd536cA4E2ff9501022b82D2319": "e50c9b9f922e5aa5e9e04968b1f49c04a1702ced4817f7be7928e13ce9ae4ae3",
	"0x5e1D2365C151aD8360fc951A7B8D376363CDFA7C": "14c17cb8f846662602bd9b9a5839f77f9be3c2bde31d7e3a3ee289aefd07a947",
	"0x24201A9f9EA8ad95c4e6d41Ed5897B6F1803be34": "ded2deb660f69e9d58c9f3d67ba5a41225149bc8bcf6021cc0def284c8aaf3cd",
	"0x4f709Fe2feE5867043e3e6dC9D8c333d766335E2": "ed1bacadd46149c5a0921114afe9d5cd509659c4d8f5b0354a73c80b0a12be8e",
	"0x7954978a42585a1f8f42033211c4B10669101434": "55dd08e683e8462a44a8298934643cc72d0db8042dceab202e552cdd30b3002",
	"0x72E710Ca2faCd748c8b6d8e3384A670d9E4db37F": "353c4d92a3f69cfff9b22cffedcb3653c0ab7011abcdce7deaa0a9cd27906b21",
	"0xcd29Da93dAcb42c159bD20CC101011BF71d6665C": "4e4757adc237cf95bc571961db27724f727d4a8238cb4f05d82bc2a9e688c118",
	"0xEE07453651943A279F3720E90EfaE4B8062fCBdc": "280b53cd44f58c8414676178da780e6ee5809517f6159967cfb992cb6ded5873",
	"0x4DD3a894eCFD053d43fa6907D87a3148D096F914": "80ade86124135c8aa9522552f70738e47c1adb0d2199246fc40be6d50a00bb8",
	"0xa002eAc0E0C7f0A04aaCAdf525D15385e62cbD1c": "11e6f4d76f1d760614bb9890281c6acfcdf0940058f0027ec473edd3b9c289b3",
	"0x8Dbb0C85766B926440Ad86B65661909a971619D6": "81c87aa7d0c0a477f8da4d206eb6a2a9f875b19cf17bfbc68665ab700e49ffbf",
	"0xCe677D240f47CE9da4ad62FDdc300D34FF5dc570": "a5d6980a09d8a385d9399ad0e90692f07da6338606726e976096a80ede1281e9",
	"0x022123Ea2beaC334d1C719dA3c50f115d5D3F66A": "97837b38075c340632012c7ff430c5f4e65ccf39141b56595f215246ebd9b556",
	"0xDDB94ACD7e6882A022A39697bf5ac91aeEbA1789": "385526d1128de13b4ff0e2464099847d879a2a87dd0f09ae88a4d6d397187cd2",
	"0x560507A3b5bD55494Bc77fAbF5b85D63a0d78343": "2fa5844a8eb63a1c07c1fa17bee31cc5ad94dca52e308c43193b4c4128861cab",
	"0x57bf6342Ce7a204dbB4516277C411B9A2769Dc30": "5551b99b3ded4cc1772d9fd6d37dd1bdd177051bc3ae830fdf443fc627d1611a",
	"0xEd5a978dDbD00f2e419b06a1Bf0A89a969B1f01F": "d399b9e2fdb149ca5739ea49c01ea670f6d0932ab3291a3be903b77c66ab8be0",
	"0xe3831bBaa2A26C81D9Ab5f7580420196c8d2cC02": "1881fb34329141144b4f37c0e7d71837c197b3467545b134fab77be03fbd3afe",
	"0x138c3D416a96a854E6EB138e55Bbb1BFCAAEed56": "829ff4f0118288d8d5251076923bf1a10c66a5fb8a2b298f155d84ae977d92bc",
	"0x17E86AcA69C12cDA9519e858411CDBC12D79F4Fc": "26bf5109220371aedc8438a9142baf88c53922950f7ecf19c9c62614cd0f0175",
	"0x594Fbfc65EE9adaeec98b9bA1D89Ea75a31AC1b5": "a1cdff7dbbdff9f6dba4fcf759f7afc2b1602d43ee1cd95dabda924c0f8ded91",
	"0x1B7648833aE1E668801252AdaCC98BD55252A079": "debb993204674fbc479fe7998a19e0934242b4b92bf780c7843197aafc36b4dc",
	"0x98B50b2B8E8e7F55bfa51652806Fa477C88DD35b": "7bf9a86a50b97cc2e69c30212eb25192a3616d02c6b91d18f3df7d287f02b6fc",
	"0x30152a7D5Eb3a07fC3Fb47100e4B1e7dE0C0DB53": "4e049370ec025ab5e4f8fa23b59c08a33cd008ed1cb2d23b41589b88c438b584",
	"0xE3e60fE8F90283F6773A2D9f59a1f53165Fa2E4B": "9ec376e192663f60285797e85cdb01c4994683e1e1cdca6ccfdba379f63336ec",
	"0x4495709B8c523E64951E79884e09577fA244d4D3": "7b5a7b26423c47e843974056834dc0dc9843b1644bad2c8c6f604d76ec3da9a3",
	"0x9561dF9D60edb6b3AB2e55D9DD0aA2b1B249E30c": "ba6f857534f6f610e0ab3dfac5c86dac9f1bad44b431fb782e16942d5cea7940",
	"0x73148af2189ED7b6C83D070e83DC3C63446B0Dbe": "95e5f7c2862a6fb878968968b67b641459f31a61890ae41d14574e9a3749e2af",
	"0x8319E9AfB49818f9279404DAd584743816FBb8b7": "3922e23a015ac4f0395bd4e4a912dad9c794699e8fd23c9a7975ff942854152a",
	"0x97a8ea69059b0eaB9B44347A71FE64fff4B1D026": "990794f116ea8881eccb6ea3af1c6c3677b617a55f9d8aec6c9dd7c8f6de01f5",
	"0xb6f19Db2117004C2bbC741A401bFDa1A0b67a908": "83ada017041a8825eb1b8e13b48d6c0d5e3746186d0ea06029ab92579b9da202",
	"0x92506572e7F54C785048Af7621198597E06E556B": "50dd041528bb1643d9e1dae1d1bf51fa919b048bc974517f1578dce531c7dbae",
	"0xf4F5CdE076b23D6b6170e609B52A9dfd2c8B1F7f": "86be38744ea03b3c9d13c3f1428dfbd7718546ba0ce09a9212d3deb15e66b4c",
	"0x2A535009CD7Fc9f0d689176F0b6b865489421b2F": "8ab153944a8b957c264cb5b375e55a501f590136b0679a6332508018c463a04c",
	"0x108aeD8B1fc761B40Ac10F6699bD7D62130F12B2": "34874cb9f9185265c79b71dc803f960937351120c7527b2fb9dbc579629a9ef2",
	"0x44353a99D102014160685D815275efa48DFcd120": "6bec1c60e1c314755428ee6e1d9635a9594d3b409c5d60a5e6d5d7c61927a2f8",
	"0x7D78fd7dCB4098BB6A1346117435425375E3c274": "9cfe7eb19b1177a155230c9136ecb6ebf83e359cf5603b9a661e6af7a4dd1ed2",
	"0x8AA28B0EBbddb233cF8631131F5d737d9FFD25b7": "6a6c52e7a2ad99f3ba729913dd7ca1186b75108b349eef7dd6b741eeda6311f6",
	"0x6f9548d11a3028Cf92b9a25819E38bA39CE07C2B": "8ab2b345847bf5bb1fa60463a55ce4213023a70304d5a1794d78e18c5a0decb",
	"0x34A230BC38e1dB04E3A8C6690D40e9B34c973457": "15d5ceb95b2bb0c0fd90b707e496945216f6fd324c9243582ccd5277ed733614",
	"0xA0EF2A72E411247756f53E8308745a2DF3E899dA": "bed656adee64604367ccdadc8178018ee9122c6f9d8c7d15d7f9d6c7245186e1",
}

var GenesisAddrKeys7 = map[string]string{
	"0x84Cb913fa39C48DC88f19eb5f02a9Ba85d46aBc6": "c3b9a3c710b4c94151ff74623ccf67f2517651fb2f80e8e09aaa1be010141d39",
	"0x7Af0a590E3bd9C2333273a83f43184691C2cA865": "e2552ec1178e7bc2b158ac8dceda02a0755ffeb99f31d646b9aef6ffee2660ef",
	"0x117aD7ca15d44a62A593F8ce586FdBea3a92E3cc": "415c3315fd0ba882770bb7356ffb881cc1ab8931b603e025baa5854a5b4e8811",
	"0xc31D73DE913C27B412112deec62152815703B840": "38511975ec9b8475fd51a3753ccd3c0448d5584ee18c624d878adbaf149802be",
	"0xF4a1A66aC7f9dec3335de0AEA48A7395297798Bc": "cd878d999a9f895d36652a3ff9d0960f19274dcf3f7b58c1c6d041c8a0fd7aac",
	"0x9823A474E3D2E0260899270FBbE547467B0ef0B3": "5d894c85d0669b0101f95288e9401b94f7e66175b80916a7b073bc9a9f675a65",
	"0x4029a3fAE2aE824df9D32464157c1ed166c94BdB": "b610ace5ff62b14678077a373d3339bdfa51095390b817589952e60047097696",
	"0xFbE134A006d8c1eC2b2142BAAF040ed04E5606dB": "c36f755650ab623ea10dde474ae5da45db90884affa22f19abd26aa7aa512368",
	"0x7EB18F9Db4De41cFBF720228FB1B6150dbe14479": "bd99686e3db726dc62edf6d4064a1164e89f7315dae4e59267705a9ec52e6a8d",
	"0xE3E22Cd75751E7876Ce51Ca33A6E83d515442A1c": "6df2010fa2fdb49528b08ef7144eb9fb77d1a58b136b627a45fbd7ea106529c2",
	"0x3A0a80d4D9e27CFA61D725d2F50aC8DB67ea9B97": "230ff7af7578f047829a9fddc22d787fe1ce0fb0e4984113d74029c0bfd85402",
	"0xd8e70997a7Ec878CfF180F895Fd1Ddb02241C543": "9110c725676f600f559a0f218054b35b9e4e0df2c91a58ff447503d257680e03",
	"0x6A52c724184Db5fFd8cabc5251053e5eEf5515b4": "1252952ef358fd6c6d92a71cee0f5e564086de3b55275e70e18f76e28b9ee8cd",
	"0x3C21f496b369F94a6a0ae369Ea8520635aef08a4": "a7ae26e4de7149bb4b4555192c33a7df122e4784692012c32c86b33e7ef4d6a9",
	"0xBFaDF9a1E1DEfA133D7ac55724F456031Cc21F2c": "4a6a56515446cbc0fb827f105af0be5284ed60db02d98bae6d5b4898b8457513",
	"0x93146961129e04D7359f931932517Dcca12b9CAd": "8085b1aab84cc13b6cf036ddf8fdc15768c51305d6a789f351dfab2413b52e1d",
	"0xE0281910eABFFfC729b9086Fa1F3Aa6C786313b1": "2cd7d16ab88f650671ac1817f449c0ad773def99dfa1f069dec9e240f5c6cc99",
	"0x0235E556118f951016B940f14f589051a9E9586f": "55b02a3b6624567b15398e2fbeba0b7afad2f25f2e27329d38e52a0eb534b184",
	"0xe1c622400Cb9E858CDf5D53aF65d7Bf6CC9ca1D1": "389c48dacc64aab11db6d5d44b789118eb055f3f90b6f881e9b555db814e8516",
	"0xcDE9a270eAF3dfb3689380dDC2d1abe0717E7922": "338098062a6505bc445806c8fe4d386b7d2abe3eea961678fc60cacdba88f948",
	"0x1Ab599477876f0C12fF7784B709de16fADfA4F0c": "236602be38bb94166c46dae037ba39ca13f8c4123f6d570896bfadb4a2e054c8",
	"0x5b6473E6264c1D81e40ed60C1f631b9dF2022A01": "789a25235546db278d9c02773775db97145d2833a842c7f9873544f2d484a3e1",
	"0x1918eFD07A6CbECB33f573c21f879eCFDF1b5a4D": "785cce2d693f96188d58ca46b6715a3e719eaf52ee3cdeadb01848026437bd9a",
	"0x481D0DC4f5E2a16f6216E6F6E00E1AB23332230A": "64c78a41f8a13db1865225fba28385b86add7705769064056a57ebc71f702d2e",
	"0x65B8AB4194AAf57F8f290E39643766F19cCC8451": "c93fb0593b657d6423b7ea620b337b25e8da3c9d03c41258dc410043d3cc164f",
	"0x22804d018B28EB93aB7dd1A45e04176109A50fA9": "21c41e13c6604d7f785198954455a2f61aba00d069fff1a13ff8491c366b127b",
	"0x61F666a0b0fd05D2014c77fb4d7825c8A026eBff": "a077d765ee2482833723c9f21da98e39717f1379ac5192c30dd5aec74aa0d6c0",
	"0x47F764EBFF4e432Ed16fa8E8AfE10cD54339D097": "87127aaa0f31a8a3eef4aad2f9eea2e464a5bb14d9273613febb838ecac037ca",
	"0xBa57FBf6350856f387745B3Dd0DeEE7acEEF2151": "4bbb76288490eed85ae0e5618e4ed174d61391dce1dcacf9c091c7c37cf76934",
	"0xD4DBf3a2FC399B232BfFF3ef27439aD98cc86E82": "7ac73b31925404e6b7d355ac42c1f4988dcbc9e06cb4722cb083abe0181df0d",
	"0x403Fae3D8DE5d5FAa015544A9ffEDEdB7abdFa7d": "3ff5cbec6563bf418972ac4608e15302e42620308366f59106686bc23229acfe",
	"0x6190B8eC760c15688aaebB0225E15ffD0D736aa6": "e8f2df33af6b8c2ccfbe42ebfebd0365cb8a977b85dface39225b0cfe46af435",
	"0x43c958e3F1986E9CD1FB18b2ff3d8C9b9d8f118E": "947a6c2e4983baa7ec3cbf1897dca350b3c6f4968b4e80f9047e6be0891390e9",
	"0x8df554bEE5d4a2B1501F3a7aeFA2496CD22d2B59": "14ca55b854fa3941aef90418023b82686f1026036a8ee93a62378d7d49cdca9a",
	"0xc1e91eF0c716fA3E1889A40d99bd4239324A371C": "aa2ad7c9e7dcc122a0bae3cb2497e152ff22b70b081f3611e645d5605ba87f57",
	"0xB6e979b9e9f5f3B72861A2D97401ca4f2cf1b527": "da9f84d59ddeb6b118c0ef0cfc67750075a8cd4776dd00d5907b0c4d618432a9",
	"0xF6da2843A6b5e444fB2F4F855c519B81756d6ccC": "749dc29aa09a05d57f9aea44268552a5580cd86659f84328cc83347b62d3d9b4",
	"0x6B614D423d3142c87a7420013740eDe1E622a31f": "c9f5757844912c5d30b317b4658216cca0b1669b27030806efee6f8f4eb34a84",
	"0x20B808694cE764823Ae193BD6C60d2076de213DE": "7ffea2eba375fc654b9a324583ba4a545df81dd098ea1ce216001b6a08c3c318",
	"0xeb3F0241f301cd336E5F364ABF5F8d48c8719520": "329dbbeb6be2546f2c314cb5c22686c3be9d2646c05cdc2c6c7e060a88d2bace",
	"0x707D95b0AAE1C02f3b41cdC86f93C81C08846018": "4a710e6cd33c2b54df3202c49ed58734e79a8c726c2c18cb7a5b47452941e468",
	"0xFD5de46aBD78f23576715c1f035f30Ec64e324fB": "153dd4630e6026883159a3cf298f4b51962ff7f0062b83d3a4803baf4e094fb2",
	"0x23dccC1BDaAb86b082FD481b954bDb3E0D152C04": "e8b698e92b90c37ce97ce811183d22f7fbd78d3a012a833d06aea1710fd0e7e8",
	"0x58C6966f739d3b6A747Cf80A506927C75C0e6E61": "6134fb48186d25e6bd534baf6dc64287d8d9fa95cf61e2e0187eff2a458ac0a2",
	"0x70d12EDE9d342AB5D919CFdB3c79b90e3D760249": "b6806f744c675e72ea6e40ff114608b92c2fe3562cc04e24136c33bd39c7691",
	"0x925628e229BA8e733A5003d53C89cc9A2cE00Baa": "5318e0ea03a8dc1d2eff5b714e9f9d8e74214645958aaba3684944387a3ce885",
	"0x926DEaacabbdC50856c82f385dc2b3B00b76401d": "f4767b3fccecd0e704c820efa9cdd8f5fc03713a7d58815dcd2e5092446fdac8",
	"0xeF9B41B1124063a9Ed92aF087224c21f00d3c5f0": "9abbf2a16f269d27b2110ec4ab8b7d5c932ea7509d10f0926332affc67292c6a",
	"0x95a0106F3aAECC8A11413b398671aC9Aa0D95D42": "cedcd965a9860b7546355e887928aeb54e0c0ab9e7b4d6ff44565d6d157d20e3",
	"0x7A55421145B95a655bEF5bB3BF98da1459411009": "69674f71c244e09c0659261ec7eb5b5af27547a009cac2e5f2b000531575a1f7",
}

var GenesisAddrKeys8 = map[string]string{
	"0x86E3efd723a75E24477FF5C7ed04BaA26dC7C413": "56091cee67e20060ed15064a2662895d6710f82747a2c9d36333d84a6b1d19e8",
	"0x4062cBE8516a43f6DdF496eA1D1bDE1AbE74B92b": "9206883d913bba4551cd1ed67c3f3fbea65e8ff46d20a147e8dd127c93e4a3ee",
	"0xf9099a0C981a2369a67260AbFe648Ad98b175Bd8": "b78946ef7fbe68e92fc81985d70d195d967996bf55c51ec90fe525a8d5129e0",
	"0xA0375d6B5EAa0d926e67312604C17AE1D97E6a87": "afe16007ee9be6d9973d9bfcde149ca0a2a0f14142437a01b01b22dff3fa94f4",
	"0xB10CC4d174c500bcf183ad0d8041BC15e3475726": "7c8354bafb9e4590af708bc43de6a7ebd07dbff97b986bd2ae2cc9ccf00c70c",
	"0xcc59CF85AE5F322BfCA84D86B422614a6B4B3386": "d73e89ad5d1afb824f3a65e04d128fc6151e29f7b7f75183017e3972064f93ab",
	"0x193E9126363253804551C501a9bBa650308Dc0ba": "704d74ea649b6809c92a14c68ef2083760a0075a96412f7e3140e78f7db13467",
	"0xD23F4B9a9f11903cbdC35076838457527C73dfCf": "b737a3b492a423a175a9a15706e890b6691c7eeaafc01033a1feab8aace77d52",
	"0x7425399E5d0aa9Aa6875B362240f09A1Fc570474": "4213702c28a6ba6cb5db223c141784dcd972561117fa758cc768588b79cb32bd",
	"0xE052B3a61460dd401B48396Cf6a273D53Ef8eD9C": "4e7781312fd9512e9111eb77b20a6db3a52b3fec9f8ae9ff58a3eeb4ed686da",
	"0x0ce4bE64CB6488Ec44B847aebd61147a1cbEcEb4": "95a38af1f6284af8024475113531130c05b6b42c86d6dae6b6dbac9df763e60c",
	"0x4aC865f68633660c8bf680D0979344646be20BA8": "588925f47a539d03d008aa446975c76ed5ff201da0ce632b794358a541ebeeb7",
	"0xdF53c52a702b6ecB21458C54E3187E69Db4b4B71": "d3e4a138a3ba21e2e7c4eee30fcc0e10b775629a7d1ec8735a6a6105e26f9cdd",
	"0x348A2F1845fa37D357ae1cF9EcFF64F86306F5c4": "7d6edfd1409a1fb8d7d4f866081cf759379cc0c5246e97b3870b797ef12490f7",
	"0x58ba21471f5F31c65665EDd4e13fEAF98B6187c0": "239a46ce98205034d03b63398db2a831a4dfd4df161693bc899475d294d09abb",
	"0x28C1727bD07bfb0FFfB3d6B8E68C51e30c072B2d": "b8038e13478416bd3e569b19a77367746b47b55004f385755ef684ff960c52bd",
	"0xaE499246EDF0fb56f1084651BeFd48bE1CEa971A": "62d442c606d6f0382199f4acb564b6c67918d0d6be772917efdf2e3de8a88555",
	"0x6a137dF08835f0DD4D145dDA259124B0C85d6fA5": "afe0f1d77dedd2e6f6854c049dbaab10168d1c29c92f1bf6c5dc326743ac29a1",
	"0xa154de2dDf028fB429A9da97dC3C801aA06dbf68": "4cfe779f4c80bb9bbf8ae2949201f139cfe83ea22959651d2e3883df72d7ce3c",
	"0x475Fad7CcD7E21099d3e2C4f51BBf8954B7B4a14": "f04dc5103530b1c91c4dd44e7e732bb671fba6aada65c91bd4442c6c70932cfb",
	"0x0F7D4042299563c360852A3d0A502a7cC6C0ec1d": "8f38a908ae56a962782c24be4dd97109b6b01029ecca58f4119648ce4ba91ee",
	"0x23f9570DB9f0a49BC65F2280FF0D48815C7e703e": "d6e0c37edfcdb23785094448201a67b3bf4bde0ddf99ba7ef886f8af3d52032b",
	"0xd87b51A274e435920f9b4655C597693eEFeab873": "ee96a5640e9821892d1c9b5e1a1c3aeb23dbd6bca96e8940f80bb2fb70b88caf",
	"0x8acb50102362b66a728338Af200ab0cEbC934043": "7ebb18038caf5fc54d4bf1300aa906f1decd47ee172308dddb38fa23dd9d73e7",
	"0x89FddDEC85A08957492A5A8F70DC5657386Badc7": "5414031fe60a784b5be368244cbfb9dc99143c9b4d8cb68851586f0d64b11ecf",
	"0x23b21e075E2fF447b931180BcE1a227b55aDeDbC": "4f03076c28a013ff1dda62b930853b249bb078d8577d13b81b17ac8b05b9cc8f",
	"0x89b74Ea833FF7e57F5BB3A73c1e7eB1F8248D2F2": "77a09fd17388ae51fded6b154d59da050a0beaba7345f32035c6d67f588074f2",
	"0xc9F654629819216A91093a579af88a3ea8FDa9Bb": "f14057e970e26c58eba3b59806c53fb620d2e52a810be84557edb34dce36f654",
	"0xf00f518ED3a71588E200BBDBe9d67b854416A08f": "3fd6ff003900aa4c291a7e49eef52024a01683153e82c91ce0fd1ec9e1010dba",
	"0xB59Bc089fb568c6dFCa37648A08658A2131c5E9c": "5133fa99d2c910180a1c9910bff9156906a63e92aeb565992343040ace22cf1a",
	"0x18a3171FDc37B633b5e25304E829535827c5BC43": "88717ee86252a1c82f5ec5f9fe4d89f45ce213bcd94567765c00c10ada51155d",
	"0xdF9FaFD68Fc08c605E96F00166C6C13c7DE1c281": "24ab46d9fed94d33178fbf9f724f76789a764b6aed3cadd8a06fe84bceed3f9f",
	"0xe523dc495a0837e71Fbcd008D1D5427880DCEb67": "6467259cde1d980b8a918db1c9ff1ce2cb086cba5912f7f5c68e9e719e3038d9",
	"0x1CaF3A8b3306A126885f1ABC7bdE376cE08fF520": "25d24240f4842755cef4fcf7454af178492b4ec60f66f2d212ee2f5a9e1b576d",
	"0xbA1815fcFFEB8fFc18591dd5587fC6CB35F244c2": "d625024f00076611e2c77da7ec311826c14e8976e23e1444444eac0fafcce826",
	"0xb335014AaFd88110CE3Af12B6C0d7AF560336827": "7a0b6782c704f4a0da51b54c5707b5e831df2a3b54fb093b6293b091e8457381",
	"0xBC37B4d5e5e562917a598260cd28759da3D89726": "36f959d8319ac6d74764335b9455f4809b6e95203e1e27f6c07721a00d9756e1",
	"0x22A38c0fCDd270f9613250f66eD4b759E7dEC5D0": "d9e8d5ff06f0254172fc6f35758de8904b9f89e4335bc2618de89ee9ac65fec8",
	"0x09887A85A68bd7089F5bA326195d3fb43E227dB5": "c3389c5ab35e6ca370a93350fe8814b3338057a3fa32527da15bf199e871a1b1",
	"0xb4C345B5E46f86D7f8c4dFCC595568CCca3972Be": "7466e53c80e15cfe2319fa4b9bf027ae542684716ccfb34c604782928b5dcf8e",
	"0x041cA2af2b3a5A912486A22b3e15C690d6f681EE": "10d9b2b8daa1a1f127298fd126422d438002789eb943e5f479b0f62a8ce9ec5b",
	"0x210dA2612B45B24cf4a98A1d63E2c4e3eC571530": "15631fb94ca5ffdebbab36defc5878647c6aed1a06be4019a94cded04f9255be",
	"0xeA343Ab026bE8418194853bF79D55dA9db654bD7": "b6a902a5ae2ab933480dac8511706c5988a6e294907aa99f119a99b9df63ab54",
	"0xABb786e04fe72f66DaA61C1C7b4ae1844753833b": "bed512ce29f94e0c36ecd8426897a2e5476b5aaa966910afb3f38bff1a68aa7",
	"0x35662fBf43E04Ca8873580bDBa77f5eB00aDC36B": "2d137f23d8b2a7fdfa1ed6822e780fe79af97141b6f29a8d0654938b2cba9d23",
	"0x6862017A9a4fcdd54954637c38Bb9906FEeaE1FB": "b99782a635d13bba54708279504f5d7358546a038c3d39e5078866515a6b3bfa",
	"0xe4a85894F36F131E076B2C6Ff1298E5424C75F9c": "b2e13f0ad29706b21c621c11792fe6bfb1dc4064b01b7cf788d8b55bf8e75fc2",
	"0xeaAd9dCb3F2c568dcA4C7BD20EaF1A9DF7201721": "3f19ee89b1ebf95ec934bc406bb8f886777ec3f07e6d0ea7c777486c6e3407cf",
	"0xF66538EB557c32eA9dde18fAf425047f45C819D5": "7dc203fa2ce00bdb5202c14b1d4b632051a522786c84834511e45f5d62112c3a",
	"0x693ba8f8F224B062d1Bf3509c719B579786592c1": "f409d8a51522f5c6b2d82ebd8d8bb6a9521197b96e2a73489b90bf2fbb32eafb",
}

var GenesisAddrKeys9 = map[string]string{
	"0x89eB0820dcB33F17a8c13006AA379BB9874B8545": "fe6d701e5b1881b39c81113326dc4b2a9eabc7c9ad0fa796a0b4a8e9e66d1c02",
	"0xb91230ccF167628Dd1bD0B26A6307826EeA669C9": "d54feeb07e2161cc9de8b679ca30c838a38707da85180cf545ef3094843eec03",
	"0x7f5A23F60C5617FD5D99CBc5703ADD6e52461643": "7be5ee9d1d284f0c84ba43f073ee4751ca683752538c82daac11c8775b8575b6",
	"0xA7e1e4B6a390dFF1eF261791f6D483882203A375": "498b4e982108121170a84161e9444947606b29f6d15851b13b08955d7153bcec",
	"0x6cfB73EDE12D3C1b98f233acD42ED0C5CdC1617F": "5f57e6aed98d68e55a7397fd2b9823b9e0b49d88e195aa6d2e618ff9a95f8514",
	"0xdD730648cddbc25F76fEeF55b18f0e5aCFa8FB99": "d7d261f30f212b2a04ee5c0de409a949a239c4aa6b744e7833cff961bb43ff88",
	"0x83B8494566d0b41385d4892394eABF6b124bFd7F": "737ea7b7b990545866c192ac68cae8d4af49d7368515c23c5cbf77b5a8eb51",
	"0xC49635c73705AE6a5f03D03Fd6e56475e36124e1": "e816ffff807ba3ab02b311e3b48b2c539827de0494d33f0c2dc7c437062712d0",
	"0x80293C6cBD4637D11C4cB3356027bbeb24da5CCB": "40ca6782a2e1b4e50883ead064070db876676cfacbf179d8cf21939be75fe938",
	"0x0b5049D03b8F45BD6EF4a0A517bC781E0634B7f4": "98d2514d5be03397c75bba8f483606582cfe589be47902fdc4d5f82106a72f12",
	"0x3Bd0d81ba5b59306a2da9E9f16905f970c798E6F": "9a0cef56e3c054343b95ba638b3b33020899a990e9c5e709c6eb9b4878e8a78d",
	"0xA4B24e023C54EF1EB079c20f5C5cfB9204C8aD58": "801e1a4d431c84d8bbd8ce724dbdd4b80619e8d157c3ba170b9df8fc58a02da0",
	"0xFdEf0e90d0647Cb17C72857cC54b4A30D4BE3F71": "66944eb245e9b28d21c5d541f7533dd82cc9d4b1be1fd156f3e56ea9e3e51a7d",
	"0x9Ac4816366Fdc47E8dA98bc448185F1184111B4c": "17fae7344ed8bfdefced328ac51bb5419871e61cb111812044ed4d6d283ead8",
	"0x6742E24d3164a7D04099cA6bA18b45DD22c867Bd": "78df24ba08b5f9152061ddd975e64c5c8d32a823f693ba4d67f1f7a8abc1aa7b",
	"0xC560147250c296Fe3827ecF57F08439B80Dc122f": "3ecabcda355c87af02952c4126d356e7436f3da1e24d9f44c7e9a6b9ce5f9e04",
	"0xC8b233F1E07BC77Ccf20019c53B498CD49Af9161": "7657f44a838e2e0058d0fc3b69a23a252fccb2d4be263f383ba904b4af120893",
	"0x2A7cbF06759905ec09E2aa767A4991D6fab4B072": "52525278517c2a2d69d680380f96cdc14d638ee9d34b01a229e2acc2ed72a60a",
	"0x654F1DF238817a9fa0b60c0A2679fb6fE77E9637": "940cda740c8db3637e34a0f32cb5310312fc1e3fd85370d84e38067472cb9191",
	"0x2248a2088519566dbFb126dD03cf185fEE30C8cc": "1b9c5b87f038db8b2034f3d5e8f617de4ec9f966b683a1febd471059500884db",
	"0xCb7E99754698d1108710F913f3e87F25C3482982": "5c882e78bf24c902d62ab322351e83f20905060f5e3fac588953a2456c291ad1",
	"0xA5938FE5843930f376649C55654465406C6E8318": "406784851ab4b73fc98ca4fb72a090ff6c771a388b1270c7a15024801bb12012",
	"0xad30e23326492Dd2F724753D096Fd7683e97914a": "6c68a699ff5c79d0a44444d5d38476b0fe28b10cb9fcb89d8aab234e9f155a6c",
	"0xc954560EfF9a9Bc5A11ca49BBC6e53164c8271Ec": "995268e468dd2561894cdb08d70dae1fa70cf7dbc71d620e199ea3b3039578e4",
	"0x8dcdf6591a00277212F5aAf436516106E3706622": "c7f36d34f47a7ad67f6f49f9abc298e9e5dd29bca363b6bdc0a10a900df96d3a",
	"0x4178920f33a7614F07C505e1F5ff9B96Cb97A2E8": "96cf1a9b16d48703e1b8843a1255bac30bef6c9e3522bf8e1c24a4fe7c6db691",
	"0xFFcFC776A0cc21B4D8Fdc9988C5a905dC5c31353": "26973839c1867ee6010fe7b0d75c8ccf29e1124b0809bd7f02a4e0287640b6d5",
	"0xB8Bef98f31F6D2D018BA9Ad70E5c454B972ea0f2": "181a9f0739b7a463bef2ad9cd2dc963dcbe1fbf93e360cf537d716565347a341",
	"0xd3eCE5d303C7E30BbC3de02C53b6Fd0993f47Ed1": "b5e4e6f14f6706aac7fc43017f03e06ed4066537151ab0de216b6beea06b4bcb",
	"0xa30c17fFC046B73e32EE23aE0DfAA169B19d294d": "e6a65f2ea989fe5b919d67dce22d59fb53edb747feb049e36a54b92c3d2f4522",
	"0x3f012ebd9ce6399BBA502e1f0a1996a65084Ec05": "a503e43979d2c84d836c80a27f7d175a2db868ca3173e8a4dba781f78a7ecf09",
	"0xff88C6f9D0dEc61795D294C56FBdd082FF224981": "66516cc9c57ee8001569657b6a18761e414282e42af3cac6a61ed67d67067f8d",
	"0xD952f3454c873CE4FD92B0cf6D8e8dd6561387aF": "e5331db764df87c887c4c310c82a7a09f995a44c68f890ea4067acc3099585ed",
	"0x8E429BebDe4E4Fa8Dfc50320464637bb21bA553F": "3a04ab7870e6d4ed9db3e378d249a9a026ca9593350936ed271966eae0d79756",
	"0x7F9021A371b6F48745eDc74b18cb8F3e3cAFEE5b": "20af4d9f7d67432ffe320ea5af6c06b04d775a598a41e6aec829a391811d55c2",
	"0x6dc4cA1482F7Ccd26dF763244DaEf5b1117032f0": "a958c7e11c17429ccb56a15210466b03dbc5b4b36674972cffc93b36ab962421",
	"0x19389C8F0B004cCD80aA259e1F66F953663C77BF": "f5e811d47eb8c95cc191473b6c7870a21b38a913e0bcef0a0e38e9c270e0dfb3",
	"0x4637Ff06a1edc71F530eED873AEbde75595C9F7f": "abdebce59cea075da5e0c42870b025ed3392dad91cddc08904ccc8dba86c72b6",
	"0x6b8Da59289372eFF5e022a7728B06A87e964FF54": "ad0483559043b45c2294d65bfba04b594c66ff4a8f8e23f8d039931ea3ab7152",
	"0xF850CDA1E45E86fbBBd7A5Eb6153a4F80715a3F5": "9978b32ff65e2c56db2233b2f6df9ea41fb389e23677d85688da8a28ee919f5d",
	"0x34e7a630ec6b8cea578ceb807838f8408Dcf0B49": "640ab55f73a1d21cae7bc482045dffca59f5a124b9d4008a470734c8b71e8d38",
	"0xfB93d7272245e394DBf3883cf238799a593dF1c6": "844ffd7569784a2f18a92c49c323c6334bde3f79c54b259ebfc371bffdf35263",
	"0x05660c24B1e4650375eC60B9482e38Ad03fE7308": "d67ec1a11fa2ddba218c7d02c361d550638b9b029505dc63494de16648849c0a",
	"0xE3523cf964460826Cb64e5F201686B98e5D7A9A3": "127bb26dac98c11aece973e5275e7f185293fcbb90681e0bfd1f6082cbd363d",
	"0x67380149f28EDe9BC8d835A42A3472eD0CBD3ed3": "fedafcfdb24dccc187616be9d65fd3c72c5070ea6921ce2db6c0c557dd15019",
	"0x45F459BBe2bC6f10B88142dE92b12B1E2B1777E9": "7024e73901388876dc3c741295862fbfd1d05ef459a77132e393dee246781c7b",
	"0xb1832cfc4f964ec532F488231c929B5fc73199Ab": "474ce73d1c354968aa4d330f2eeebb55acfadf0c8c4c8529b6fd35685f972f4c",
	"0x79AAABf09d8Fa74B592d55Cb53E724eB084AFf0e": "37dfe153116c9379540160abbeb755a21f9f913c03598e0568e0449f975e4e10",
	"0xD3F1591df8AcA6d151B2845Fe6e28a14c03De112": "202e88e81575ef0529039d7577edfbc0d1de6e42e63deb3df0a29ab52a9bcae4",
	"0xe4F96004B59752fdb8d5EC76B6225999cF905C47": "b701ca66cfc14c3755bc4123c888ac313f49753632d1050ff7250c7680dc1a46",
}

var GenesisAddrKeys10 = map[string]string{
	"0x5228abE14e84A93C0f6A6d68D75a9D56eC6c34E8": "d83a98a8155a39c35dac606a555822e8053110e759c108d85e0fbcb941073882",
	"0xB300f8B1B58173128c7089412d924EBfEaF2213d": "dc7a5ca7925b33bc14db6ee0e6f50c46e6eadb4ea9b6b452f7c82a7cef7f30bd",
	"0xb028B14f3636ef04896d13F105B55DAfAF69ad6C": "8127844a73cb443626628ac53eacd1f1164b615f0714588cff10942d19b04c18",
	"0x60EfdF9c74EB529034d81E08860e8277ed530447": "6560b2478173846795ca3b33af373e09a41fe9741fa9ef00a7658bc28d0c8af7",
	"0x7cD2cb3adDd50731Dd6bbe695358FcCAd3FFC49c": "b058ad8c4aec9c7a3d6216b6a127c93575218a77ddde2ff65a89ece0ccbed4d3",
	"0xb2c9F95e13D2c7468329802C6c3ec42ff55EB987": "c4d25dfcb2df4dc7f68d7cb654899f09058afcf692530f9fa3eb962b3223a522",
	"0xF3cfC404ABc52bd8c5b0fe083036bf36c7c7c1e9": "52cf88609198f0201d349d9c724cae33fdf47c9bdd0761c8a69a088f2132219d",
	"0xe130C2cfE9e8a890D13420b0E0F2a6fc398158d1": "591228c439e23605f96947f47c79ec706a194c194290a62493af07eeffe1bad4",
	"0xA1da50f7C51afc74e83c85b07fc81659Eb473df6": "3a09b422928630c487b692da6898ed9c5d4c6aec82d985868d721b2f5660e47",
	"0xB7005bd9f050129624c852239c1027f0EB3C9490": "8406051fdd437008545460adb56e6e9fc31d90daf30aabbdc4890004719568c3",
	"0xD5028A7E2Cc20b4b1417fe61aFd0FbF8f3f668A9": "45dc78cf347fda73406f688d2b581fd5d1d5c3b817e3fc0e6c1d38686b413d01",
	"0x4c9F2C42e76403f02036125481EE3fb9A5a6F3af": "2d09d05e3df14ec2a98ce245a9a0f286231f3ac461e118be3587f2706c6d8d71",
	"0x6f4aD992bD7452dA6Bc04b49eEF4fF56aed80A20": "cc182aa73a2294077d83bc0441384750e371b39f9ef53e6a6156fc46bff2d48e",
	"0x0FD0E675fB245A92b6Dfda8d6a9C1F53A974627c": "e5da70934ea656147e7185140bef3a1e9066375fe57297d8e9b0a11a75e125a2",
	"0x4331222474cC224821A9B5057F599E45fc71f13A": "69d72ff6dc6160008da8fd338162cd02b0ada94297069005288bd710970e7c60",
	"0xBDEC05D84DADcA94586d254b25300fdfaA4f6a32": "44db95db032ebb29ba61bdbbd30bd8b405aa106a582c59982fa53c891f88c62c",
	"0xcDD679400202044588d3710F333C2447aFF0b7f4": "a18a0014ffce06b229f383208d89f1cedf331ebcddb4ce94f1048a24f6acd43b",
	"0xE3a894E2cD3c265ccDd92AdEa4bD049F421f331c": "97494ddff32d721581afe87b7b1b51af039f0e197d19b22d3fa9b9208462d792",
	"0xc79C647d43b1b7f52bbBD67010c7687071b5972a": "ff5c18eca8ba36342a5494f7b6a16ae00ab00b400325bf77397cc64db781336b",
	"0xc2172E792A0419ed985246F550905c1EeAf418AC": "f9c143b489e00aa54a0a1d63a2f61400865543a0319f5c370b49c964fc07fd86",
	"0xFC0311C99d19aB91ab1d673D3703a96E646a058C": "6dded5634b347c7d64de4621c6781ebc730cc11693e54a2a982e4aa3bfcce3a8",
	"0x2e3AA386Eb3C35dFD9354dC6bCdf8a69Fa17B1f0": "cc1204a455b0a7bac994e108f56d5633a80f5243f5fc26112152868760c742f9",
	"0xEfD973009B580efC83025eA4CD57F173292AFbE5": "a06d72d4a1777f784b58d9e8df7ca37fca444c48cd6f99d93b0f3d18136dd85",
	"0x43BC478daeD5986B6B6FBC5208F39e71D764ea10": "c8fa192970a6e2dc45d47ca85b8a4d7a6159f4a03ca6b5adf64aff7788f80a49",
	"0x8C8C9CE45b73C4F28CC5d6B3E47151fC501b8385": "27b56dcc60383fd3256c5f3b34182d07db46355f61e8dbd872345f930a9959f3",
	"0x62201103C4ea45129114979a478166a6fC90f0fb": "cd67c5c8bad3809fc90eb8995f7ca89ab6a364439b2d4453b2a781a6f67b99a0",
	"0x059bC0d009E42731eF093866909C6E0828380667": "92b18a5ab59c7c46a6f8dfeb6a1f49275a75b12b0be883bdc0715dd175a98be8",
	"0x71aBfcc9fFc52Ce89D6d2346a366d8c1689e5f34": "91064a448f713737ecd6554c6c194cec531aab7e444f556b57d1139ea663439d",
	"0x97D7a80732C778FA847B038F0B10ECB4647D3ca8": "f9ae6a431f498ba0b1b75815fa4a387b88029070f13a5a2d24f01e1ad112144",
	"0xE688D2960b8e7a5F67e920C6A895aa15edc0f196": "93db655389a24038602f2b90358f6f069d50660d42ed9d63564d609f7c8fc37a",
	"0x70802BCB7e132d351d4852D2EC54eD997D500973": "b186f77a67a78d2f86519093374b8b674ffa0510d58085d0870b6e8a1779e113",
	"0xaFb2f647F4B7580BE588fbff8eC30d238B7b132B": "1c394aee17109819e67ddf1d56db15ce3bd4b89859436911150d01b34c19c0d6",
	"0x928Fe7a442Fb5be198A7062D0b5059b5B7E75111": "ca585756bf55423f58c052a1c886ccde09343bc0c362f92dcdeeda85d07e99d",
	"0xb441C2C665eC4af34D92b699b4F26a42a8041717": "d111e041d7495d13ed47023e261d95599ab9ef5b0976e2b8f3511a7c1ad3d166",
	"0x43060d366921aF2009E294Df025d1c774b951039": "3f22767d01dbafb3195d911226813f18d8708d4c06d3b8afe7818c3934c67cb2",
	"0xBF898F4fBF032F84989c0BFf289F7B62fB254C85": "9a4b3305f7586f71fbd4cbec0745fb62843bb3484f75fa3a81850b2fb55b53a8",
	"0xe440c505101317BF3Fc725F51E663fF56aa91b79": "6354d9a83d03cf3597419272196104dc578c58b108307b0953f67764c9f6a9d5",
	"0x82d389AaE6c9d70A1C1b74f723a4d6C5DE375Da4": "13c64a343bc8c824cf63669879a0f134694069f381714fb47136280da2dfec6d",
	"0xEb6E053C68bee56223E49927d7713E37Fd717e78": "a1d6fc5eae82ec83cc5c1f2568efb6b5af8cee3c305bc713513222a501dbb34e",
	"0x3d1ED940D2C38206F25a7DEE45832B85535cae7d": "2254d46911be5bf89c9f108998b952c8bef073bc77cff2454cdcee38960435b7",
	"0x610699A54024464F88A4E6bA7E3F228169811e2D": "a4b710ca1ddb7d078ba18ba1dea30ce8d270a11969c0e8887ff3586cbe9ad34f",
	"0xC54640492751693B2b66D2FAFF535a31b4730573": "d59ca06f2ebae8fdc58db8b2c3913f47d925339eae4fed06b608f018edaf20e9",
	"0xdf131199ac450112A530d64D737Ce18eC5FF492c": "fda05f43985dd7a4bf8714cbf97dcffdcdacbb9126d837e95c655147da3630ba",
	"0x75873231220dB8D6aCC260aB9354a22F422c4eE9": "8763dc64d808f731b6adddc644fceab6de23650eaefdecac2c8a86b2682cfe43",
	"0x02c562ff34F84c8869DD9AA719b3747cC26bF44c": "24a30ee508148d5d82ea6ffdb5b8d2c66044510519f9164eccd6382b34ff958e",
	"0x1E9d234300611F406af5859d030cB70f0d95370b": "f19b36094be402dc38e4edee4afaa072dcfa124afac9e12859e447f66ffec93e",
	"0x3C023E632A7a727f8184ca3f0A979D8ecd2a8f8e": "c60f2a565a2de754b3af677c957386e3aacdb0f007b8107f87bdd99f420b059c",
	"0x1EfEf012192fe2193bCC279907f50B4bB2ab110B": "21555fe2d8d65181e9747116363c4b29d54576ba9a8700acc1459a71c699514f",
	"0xd12739cDb159E31A1fAe0742493dCAD6Fd031d13": "9e212a0c84b85a55fa87f26548628cf5315c6f7689196d4e0336fc25b9d777f2",
	"0xDeBcB4e000F54B9fF9A81322d744718f83b4ce76": "590aded8bff67909bb3dcec9ae1c9ccbbde9433e7d553317c29e844e73408587",
}

var GenesisContractAddress = []string{
	"0x00000000000000000000000000000000736D6332",
	"0x00000000000000000000000000000000736D6331",
	"0x00000000000000000000000000000000736D6333",
	"0x00000000000000000000000000000000736D6339",
	"0x00000000000000000000000000000000736D6336",
	"0x00000000000000000000000000000000736D6337",
	"0x00000000000000000000000000000000736D6338",
}

// GenesisContract are used to initialize contract in genesis block
var GenesisContracts = map[string]string{
	//"0x00000000000000000000000000000000736d6331": "60806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166324b8ba5f8114604d5780636d4ce63c146067575b600080fd5b348015605857600080fd5b50606560ff60043516608f565b005b348015607257600080fd5b50607960a5565b6040805160ff9092168252519081900360200190f35b6000805460ff191660ff92909216919091179055565b60005460ff16905600a165627a7a723058206cc1a54f543612d04d3f16b0bbb49e9ded9ccf6d47f7789fe3577260346ed44d0029",
	// Simple voting contract bytecode in genesis block, source code in kvm/smc/Ballot.sol
	"0x00000000000000000000000000000000736D6332": "608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063124474a71461005c578063609ff1bd146100a0578063b3f98adc146100d1575b600080fd5b34801561006857600080fd5b5061008a600480360381019080803560ff169060200190929190505050610101565b6040518082815260200191505060405180910390f35b3480156100ac57600080fd5b506100b5610138565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100dd57600080fd5b506100ff600480360381019080803560ff16906020019092919050505061019e565b005b600060048260ff161015156101195760009050610133565b60018260ff1660048110151561012b57fe5b016000015490505b919050565b6000806000809150600090505b60048160ff161015610199578160018260ff1660048110151561016457fe5b0160000154111561018c5760018160ff1660048110151561018157fe5b016000015491508092505b8080600101915050610145565b505090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060000160009054906101000a900460ff1680610201575060048260ff1610155b1561020b5761026a565b60018160000160006101000a81548160ff021916908315150217905550818160000160016101000a81548160ff021916908360ff1602179055506001808360ff1660048110151561025857fe5b01600001600082825401925050819055505b50505600a165627a7a72305820c93a970449b32fe53b59e0ed7cfeda5d52acafd2d1bdd3f2f67093f076acf1c60029",
	// Counter contract bytecode in genesis block, source code in kvm/smc/SimpleCounter.sol
	"0x00000000000000000000000000000000736D6331": "6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806324b8ba5f14604e5780636d4ce63c14607b575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060a9565b005b348015608657600080fd5b50608d60c6565b604051808260ff1660ff16815260200191505060405180910390f35b806000806101000a81548160ff021916908360ff16021790555050565b60008060009054906101000a900460ff169050905600a165627a7a7230582083f88bef40b78ed8ab5f620a7a1fb7953640a541335c5c352ff0877be0ecd0c60029",
	// Exchange master contract bytecode in genesis block, source code in kvm.smc/Exchange.sol
	"0x00000000000000000000000000000000736D6333": "60806040526004361061008d5763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630a0306b18114610092578063323a9243146100b95780633c3c9c23146100d357806344af18e8146100e8578063613d03af146101005780636e63987d1461011557806386dca3341461012d578063fa8513de14610145575b600080fd5b34801561009e57600080fd5b506100a761015a565b60408051918252519081900360200190f35b3480156100c557600080fd5b506100d1600435610179565b005b3480156100df57600080fd5b506100a7610194565b3480156100f457600080fd5b506100d160043561019a565b34801561010c57600080fd5b506100a76101b5565b34801561012157600080fd5b506100d16004356101bb565b34801561013957600080fd5b506100d16004356101c6565b34801561015157600080fd5b506100a76101d1565b600060015460005411156101715750600154610176565b506000545b90565b60015481111561018857600080fd5b60018054919091039055565b60005481565b6000548111156101a957600080fd5b60008054919091039055565b60015481565b600180549091019055565b600080549091019055565b6000805460015411156101e75750600054610176565b50600154905600a165627a7a723058203f7b9ba72392daf2bb6f8a91c0d4a8a3dcd58decc81ffc4fd90951f41cb9490c0029",
	// New exchange master contract bytecode in genesis block allows multiple recipients, source code in kvm.smc/ExchangeV2.sol
	"0x00000000000000000000000000000000736D6339": "6080604052600436106100f1576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631ccae936146100f657806340caae061461028b578063414aff2b146102a257806342d4d52a1461036557806345ea12aa146104475780634db0315a1461048a57806360c76dbc1461056c578063893d20e8146106da57806397175ddb14610731578063a649803214610859578063c109a06e1461089c578063e3f2804f1461097e578063e439799b14610a48578063f147cf5714610ac9578063fd36865114610b78578063fe40a89014610c27578063fe9fbb8014610cd6575b600080fd5b34801561010257600080fd5b50610289600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192908035906020019092919080359060200190929190505050610d31565b005b34801561029757600080fd5b506102a06116f2565b005b3480156102ae57600080fd5b50610363600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192908035906020019092919080359060200190929190505050611775565b005b34801561037157600080fd5b506103cc600480360381019080803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506119c0565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561040c5780820151818401526020810190506103f1565b50505050905090810190601f1680156104395780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561045357600080fd5b50610488600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050611d7f565b005b34801561049657600080fd5b506104f1600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050611ec4565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610531578082015181840152602081019050610516565b50505050905090810190601f16801561055e5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561057857600080fd5b5061065f600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506121b5565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561069f578082015181840152602081019050610684565b50505050905090810190601f1680156106cc5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156106e657600080fd5b506106ef61280e565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561073d57600080fd5b506107de600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050612837565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561081e578082015181840152602081019050610803565b50505050905090810190601f16801561084b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561086557600080fd5b5061089a600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050612e26565b005b3480156108a857600080fd5b50610903600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050612f6a565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610943578082015181840152602081019050610928565b50505050905090810190601f1680156109705780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561098a57600080fd5b50610a2b600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050612fb5565b604051808381526020018281526020019250505060405180910390f35b348015610a5457600080fd5b50610aaf600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050613267565b604051808215151515815260200191505060405180910390f35b348015610ad557600080fd5b50610b76600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506132e9565b005b348015610b8457600080fd5b50610c25600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f016020809104026020016040519081016040528093929190818152602001838380828437820191505050505050919291929050505061346d565b005b348015610c3357600080fd5b50610cd4600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050613634565b005b348015610ce257600080fd5b50610d17600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506137fd565b604051808215151515815260200191505060405180910390f35b610d39616222565b6000610d43616286565b60011515600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff1615151480610dee57506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b1515610e88576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f4f6e6c792061646d696e2063616e2063616c6c20746869732066756e6374696f81526020017f6e2e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b610180604051908101604052808b81526020018a81526020018981526020018881526020018781526020016020604051908101604052806000815250815260200160206040519081016040528060008152508152602001868152602001868152602001600081526020018581526020016001151581525092506006866040518082805190602001908083835b602083101515610f395780518252602082019150602081019050602083039250610f14565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff1615610f84576116e6565b60038a6040518082805190602001908083835b602083101515610fbc5780518252602082019150602081019050602083039250610f97565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020896040518082805190602001908083835b6020831015156110255780518252602082019150602081019050602083039250611000565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020839080600181540180825580915050906001820390600052602060002090600c02016000909192909190915060008201518160000190805190602001906110a09291906162af565b5060208201518160010190805190602001906110bd9291906162af565b5060408201518160020190805190602001906110da9291906162af565b5060608201518160030190805190602001906110f79291906162af565b5060808201518160040190805190602001906111149291906162af565b5060a08201518160050190805190602001906111319291906162af565b5060c082015181600601908051906020019061114e9291906162af565b5060e082015181600701556101008201518160080155610120820151816009015561014082015181600a015561016082015181600b0160006101000a81548160ff0219169083151502179055505050506004886040518082805190602001908083835b6020831015156111d657805182526020820191506020810190506020830392506111b1565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390208a6040518082805190602001908083835b60208310151561123f578051825260208201915060208101905060208303925061121a565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020896040518082805190602001908083835b6020831015156112a85780518252602082019150602081019050602083039250611283565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390208054905091506004886040518082805190602001908083835b60208310151561131957805182526020820191506020810190506020830392506112f4565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390208a6040518082805190602001908083835b602083101515611382578051825260208201915060208101905060208303925061135d565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020896040518082805190602001908083835b6020831015156113eb57805182526020820191506020810190506020830392506113c6565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020839080600181540180825580915050906001820390600052602060002090600c02016000909192909190915060008201518160000190805190602001906114669291906162af565b5060208201518160010190805190602001906114839291906162af565b5060408201518160020190805190602001906114a09291906162af565b5060608201518160030190805190602001906114bd9291906162af565b5060808201518160040190805190602001906114da9291906162af565b5060a08201518160050190805190602001906114f79291906162af565b5060c08201518160060190805190602001906115149291906162af565b5060e082015181600701556101008201518160080155610120820151816009015561014082015181600a015561016082015181600b0160006101000a81548160ff0219169083151502179055505050506080604051908101604052808981526020018b81526020018a8152602001838152509050806007876040518082805190602001908083835b6020831015156115c1578051825260208201915060208101905060208303925061159c565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060008201518160000190805190602001906116109291906162af565b50602082015181600101908051906020019061162d9291906162af565b50604082015181600201908051906020019061164a9291906162af565b506060820151816003015590505060016006876040518082805190602001908083835b602083101515611692578051825260208201915060208101905060208303925061166d565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060006101000a81548160ff0219169083151502179055506116e583613853565b5b50505050505050505050565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16141561177357336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505b565b60011515600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff161515148061182057506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b15156118ba576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f4f6e6c792061646d696e2063616e2063616c6c20746869732066756e6374696f81526020017f6e2e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b6040805190810160405280838152602001828152506005856040518082805190602001908083835b60208310151561190757805182526020820191506020810190506020830392506118e2565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020846040518082805190602001908083835b602083101515611970578051825260208201915060208101905060208303925061194b565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600082015181600001556020820151816001015590505050505050565b6060806008836040518082805190602001908083835b6020831015156119fb57805182526020820191506020810190506020830392506119d6565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020805480602002602001604051908101604052809291908181526020016000905b82821015611b0a578382906000526020600020018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015611af65780601f10611acb57610100808354040283529160200191611af6565b820191906000526020600020905b815481529060010190602001808311611ad957829003601f168201915b505050505081526020019060010190611a4e565b505050509050600081511415611b325760206040519081016040528060008152509150611d79565b806000815181101515611b4157fe5b90602001906020020151816001815181101515611b5a57fe5b90602001906020020151826002815181101515611b7357fe5b90602001906020020151836003815181101515611b8c57fe5b906020019060200201516040516020018085805190602001908083835b602083101515611bce5780518252602082019150602081019050602083039250611ba9565b6001836020036101000a038019825116818451168082178552505050505050905001807f7c0000000000000000000000000000000000000000000000000000000000000081525060010184805190602001908083835b602083101515611c495780518252602082019150602081019050602083039250611c24565b6001836020036101000a038019825116818451168082178552505050505050905001807f7c0000000000000000000000000000000000000000000000000000000000000081525060010183805190602001908083835b602083101515611cc45780518252602082019150602081019050602083039250611c9f565b6001836020036101000a038019825116818451168082178552505050505050905001807f7c0000000000000000000000000000000000000000000000000000000000000081525060010182805190602001908083835b602083101515611d3f5780518252602082019150602081019050602083039250611d1a565b6001836020036101000a03801982511681845116808217855250505050505090500194505050505060405160208183030381529060405291505b50919050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515611e69576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260218152602001807f4f6e6c7920726f6f742063616e2063616c6c20746869732066756e6374696f6e81526020017f2e0000000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690831515021790555050565b60606040516020018060000190506040516020818303038152906040526040518082805190602001908083835b602083101515611f165780518252602082019150602081019050602083039250611ef1565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040518091039020600019166002836040518082805190602001908083835b602083101515611f7f5780518252602082019150602081019050602083039250611f5a565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060405160200180828054600181600116156101000203166002900480156120105780601f10611fee576101008083540402835291820191612010565b820191906000526020600020905b815481529060010190602001808311611ffc575b50509150506040516020818303038152906040526040518082805190602001908083835b6020831015156120595780518252602082019150602081019050602083039250612034565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191614151561219c576002826040518082805190602001908083835b6020831015156120c957805182526020820191506020810190506020830392506120a4565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390208054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156121905780601f1061216557610100808354040283529160200191612190565b820191906000526020600020905b81548152906001019060200180831161217357829003601f168201915b505050505090506121b0565b602060405190810160405280600081525090505b919050565b60606128056004856040518082805190602001908083835b6020831015156121f257805182526020820191506020810190506020830392506121cd565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020846040518082805190602001908083835b60208310151561225b5780518252602082019150602081019050602083039250612236565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020836040518082805190602001908083835b6020831015156122c4578051825260208201915060208101905060208303925061229f565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020805480602002602001604051908101604052809291908181526020016000905b828210156127fc57838290600052602060002090600c02016101806040519081016040529081600082018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156123d55780601f106123aa576101008083540402835291602001916123d5565b820191906000526020600020905b8154815290600101906020018083116123b857829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156124775780601f1061244c57610100808354040283529160200191612477565b820191906000526020600020905b81548152906001019060200180831161245a57829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156125195780601f106124ee57610100808354040283529160200191612519565b820191906000526020600020905b8154815290600101906020018083116124fc57829003601f168201915b50505050508152602001600382018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156125bb5780601f10612590576101008083540402835291602001916125bb565b820191906000526020600020905b81548152906001019060200180831161259e57829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561265d5780601f106126325761010080835404028352916020019161265d565b820191906000526020600020905b81548152906001019060200180831161264057829003601f168201915b50505050508152602001600582018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156126ff5780601f106126d4576101008083540402835291602001916126ff565b820191906000526020600020905b8154815290600101906020018083116126e257829003601f168201915b50505050508152602001600682018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156127a15780601f10612776576101008083540402835291602001916127a1565b820191906000526020600020905b81548152906001019060200180831161278457829003601f168201915b50505050508152602001600782015481526020016008820154815260200160098201548152602001600a8201548152602001600b820160009054906101000a900460ff16151515158152505081526020019060010190612317565b50505050614011565b90509392505050565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b6060612e1e6003846040518082805190602001908083835b602083101515612874578051825260208201915060208101905060208303925061284f565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020836040518082805190602001908083835b6020831015156128dd57805182526020820191506020810190506020830392506128b8565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020805480602002602001604051908101604052809291908181526020016000905b82821015612e1557838290600052602060002090600c02016101806040519081016040529081600082018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156129ee5780601f106129c3576101008083540402835291602001916129ee565b820191906000526020600020905b8154815290600101906020018083116129d157829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015612a905780601f10612a6557610100808354040283529160200191612a90565b820191906000526020600020905b815481529060010190602001808311612a7357829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015612b325780601f10612b0757610100808354040283529160200191612b32565b820191906000526020600020905b815481529060010190602001808311612b1557829003601f168201915b50505050508152602001600382018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015612bd45780601f10612ba957610100808354040283529160200191612bd4565b820191906000526020600020905b815481529060010190602001808311612bb757829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015612c765780601f10612c4b57610100808354040283529160200191612c76565b820191906000526020600020905b815481529060010190602001808311612c5957829003601f168201915b50505050508152602001600582018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015612d185780601f10612ced57610100808354040283529160200191612d18565b820191906000526020600020905b815481529060010190602001808311612cfb57829003601f168201915b50505050508152602001600682018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015612dba5780601f10612d8f57610100808354040283529160200191612dba565b820191906000526020600020905b815481529060010190602001808311612d9d57829003601f168201915b50505050508152602001600782015481526020016008820154815260200160098201548152602001600a8201548152602001600b820160009054906101000a900460ff16151515158152505081526020019060010190612930565b50505050614253565b905092915050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515612f10576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260218152602001807f4f6e6c7920726f6f742063616e2063616c6c20746869732066756e6374696f6e81526020017f2e0000000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b60018060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690831515021790555050565b6060612f74616222565b612f7d836144c0565b90508061016001511515612fa35760206040519081016040528060008152509150612faf565b612fac81614bfe565b91505b50919050565b60008060006005856040518082805190602001908083835b602083101515612ff25780518252602082019150602081019050602083039250612fcd565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020846040518082805190602001908083835b60208310151561305b5780518252602082019150602081019050602083039250613036565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060000154141515613252576005846040518082805190602001908083835b6020831015156130d157805182526020820191506020810190506020830392506130ac565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020836040518082805190602001908083835b60208310151561313a5780518252602082019150602081019050602083039250613115565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600001546005856040518082805190602001908083835b6020831015156131a95780518252602082019150602081019050602083039250613184565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020846040518082805190602001908083835b60208310151561321257805182526020820191506020810190506020830392506131ed565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390206001015491509150613260565b600080819150809050915091505b9250929050565b60006006826040518082805190602001908083835b6020831015156132a1578051825260208201915060208101905060208303925061327c565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff169050919050565b6132f1616222565b60011515600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff161515148061339c57506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b1515613436576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f4f6e6c792061646d696e2063616e2063616c6c20746869732066756e6374696f81526020017f6e2e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b61343f83613267565b151561344a57613468565b613453836144c0565b9050818160a00181905250613467816151ca565b5b505050565b613475616222565b60011515600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff161515148061352057506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b15156135ba576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f4f6e6c792061646d696e2063616e2063616c6c20746869732066756e6374696f81526020017f6e2e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b6135c383613267565b15156135ce5761362f565b6135d7836144c0565b905061361d8160c00151836040805190810160405280600181526020017f2c000000000000000000000000000000000000000000000000000000000000008152506154d3565b8160c0018190525061362e816151ca565b5b505050565b60011515600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff16151514806136df57506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b1515613779576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f4f6e6c792061646d696e2063616e2063616c6c20746869732066756e6374696f81526020017f6e2e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b806002836040518082805190602001908083835b6020831015156137b2578051825260208201915060208101905060208303925061378d565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902090805190602001906137f892919061632f565b505050565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b6000806000613860616222565b60008561010001519450600386602001516040518082805190602001908083835b6020831015156138a65780518252602082019150602081019050602083039250613881565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902086600001516040518082805190602001908083835b60208310151561391357805182526020820191506020810190506020830392506138ee565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902093506000848054905010151561400957600092505b8380549050831015613f4c57600085141561397557613f4c565b6000848481548110151561398557fe5b90600052602060002090600c02016008015414156139a257613f3f565b613a6284848154811015156139b357fe5b90600052602060002090600c02016004018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015613a585780601f10613a2d57610100808354040283529160200191613a58565b820191906000526020600020905b815481529060010190602001808311613a3b57829003601f168201915b50505050506144c0565b915084826101000151101515613ce057613a8f866080015183602001518460600151888660800151615758565b84826101000151038261010001818152505060008261010001511415613abc576001826101200181815250505b81600387602001516040518082805190602001908083835b602083101515613af95780518252602082019150602081019050602083039250613ad4565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902087600001516040518082805190602001908083835b602083101515613b665780518252602082019150602081019050602083039250613b41565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902084815481101515613ba657fe5b90600052602060002090600c02016000820151816000019080519060200190613bd09291906162af565b506020820151816001019080519060200190613bed9291906162af565b506040820151816002019080519060200190613c0a9291906162af565b506060820151816003019080519060200190613c279291906162af565b506080820151816004019080519060200190613c449291906162af565b5060a0820151816005019080519060200190613c619291906162af565b5060c0820151816006019080519060200190613c7e9291906162af565b5060e082015181600701556101008201518160080155610120820151816009015561014082015181600a015561016082015181600b0160006101000a81548160ff021916908315150217905550905050613cd7826151ca565b60009450613f4c565b8161010001518503945060008261010001818152505060018261012001818152505081600387602001516040518082805190602001908083835b602083101515613d3f5780518252602082019150602081019050602083039250613d1a565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902087600001516040518082805190602001908083835b602083101515613dac5780518252602082019150602081019050602083039250613d87565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902084815481101515613dec57fe5b90600052602060002090600c02016000820151816000019080519060200190613e169291906162af565b506020820151816001019080519060200190613e339291906162af565b506040820151816002019080519060200190613e509291906162af565b506060820151816003019080519060200190613e6d9291906162af565b506080820151816004019080519060200190613e8a9291906162af565b5060a0820151816005019080519060200190613ea79291906162af565b5060c0820151816006019080519060200190613ec49291906162af565b5060e082015181600701556101008201518160080155610120820151816009015561014082015181600a015561016082015181600b0160006101000a81548160ff021916908315150217905550905050613f358660800151836020015184606001518560e001518660800151615758565b613f3e826151ca565b5b828060010193505061395b565b600085876101000151031180613f6d57508561010001518587610100015103145b1561400857848661010001510390506000811415613fca57600186610120018181525050600086610100018181525050613fa8866000615bcb565b613fc5866080015187602001518860600151888a60800151615758565b613ffe565b8486610100018181525050613fe0866000615bcb565b613ffd866080015187602001518860600151848a60800151615758565b5b614007866151ca565b5b5b505050505050565b606080600060606000855111156142375760206040519081016040528060008152509250600091505b845182101561422f57614063858381518110151561405457fe5b90602001906020020151614bfe565b90506040516020018060000190506040516020818303038152906040526040518082805190602001908083835b6020831015156140b55780518252602082019150602081019050602083039250614090565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916816040516020018082805190602001908083835b60208310151561411f57805182526020820191506020810190506020830392506140fa565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b6020831015156141885780518252602082019150602081019050602083039250614163565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040518091039020600019161415156142225761421f836141e487858151811015156141d557fe5b90602001906020020151614bfe565b6040805190810160405280600181526020017f7c000000000000000000000000000000000000000000000000000000000000008152506154d3565b92505b818060010192505061403a565b82935061424b565b602060405190810160405280600081525093505b505050919050565b606080600060606000855111156144a45760206040519081016040528060008152509250600091505b845182101561449c576000858381518110151561429557fe5b90602001906020020151610100015114156142af5761448f565b6142cf85838151811015156142c057fe5b90602001906020020151614bfe565b90506040516020018060000190506040516020818303038152906040526040518082805190602001908083835b60208310151561432157805182526020820191506020810190506020830392506142fc565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916816040516020018082805190602001908083835b60208310151561438b5780518252602082019150602081019050602083039250614366565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b6020831015156143f457805182526020820191506020810190506020830392506143cf565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191614151561448e5761448b83614450878581518110151561444157fe5b90602001906020020151614bfe565b6040805190810160405280600181526020017f7c000000000000000000000000000000000000000000000000000000000000008152506154d3565b92505b5b818060010192505061427c565b8293506144b8565b602060405190810160405280600081525093505b505050919050565b6144c8616222565b6000806006846040518082805190602001908083835b60208310151561450357805182526020820191506020810190506020830392506144de565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060009054906101000a900460ff16151561454f57614bf7565b6007846040518082805190602001908083835b6020831015156145875780518252602082019150602081019050602083039250614562565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390209150600482600001604051808280546001816001161561010002031660029004801561461d5780601f106145fb57610100808354040283529182019161461d565b820191906000526020600020905b815481529060010190602001808311614609575b5050915050908152602001604051809103902082600101604051808280546001816001161561010002031660029004801561468f5780601f1061466d57610100808354040283529182019161468f565b820191906000526020600020905b81548152906001019060200180831161467b575b505091505090815260200160405180910390208260020160405180828054600181600116156101000203166002900480156147015780601f106146df576101008083540402835291820191614701565b820191906000526020600020905b8154815290600101906020018083116146ed575b50509150509081526020016040518091039020826003015481548110151561472557fe5b90600052602060002090600c02019050806101806040519081016040529081600082018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156147dc5780601f106147b1576101008083540402835291602001916147dc565b820191906000526020600020905b8154815290600101906020018083116147bf57829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561487e5780601f106148535761010080835404028352916020019161487e565b820191906000526020600020905b81548152906001019060200180831161486157829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156149205780601f106148f557610100808354040283529160200191614920565b820191906000526020600020905b81548152906001019060200180831161490357829003601f168201915b50505050508152602001600382018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156149c25780601f10614997576101008083540402835291602001916149c2565b820191906000526020600020905b8154815290600101906020018083116149a557829003601f168201915b50505050508152602001600482018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015614a645780601f10614a3957610100808354040283529160200191614a64565b820191906000526020600020905b815481529060010190602001808311614a4757829003601f168201915b50505050508152602001600582018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015614b065780601f10614adb57610100808354040283529160200191614b06565b820191906000526020600020905b815481529060010190602001808311614ae957829003601f168201915b50505050508152602001600682018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015614ba85780601f10614b7d57610100808354040283529160200191614ba8565b820191906000526020600020905b815481529060010190602001808311614b8b57829003601f168201915b50505050508152602001600782015481526020016008820154815260200160098201548152602001600a8201548152602001600b820160009054906101000a900460ff16151515158152505092505b5050919050565b60608161016001511515614c2457602060405190810160405280600081525090506151c5565b8160000151826020015183604001518460600151614c458660e001516160cb565b614c538761010001516160cb565b87608001518860a001518960c00151614c708b61014001516160cb565b614c7e8c61012001516160cb565b604051602001808c805190602001908083835b602083101515614cb65780518252602082019150602081019050602083039250614c91565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b000000000000000000000000000000000000000000000000000000000000008152506001018b805190602001908083835b602083101515614d315780518252602082019150602081019050602083039250614d0c565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b000000000000000000000000000000000000000000000000000000000000008152506001018a805190602001908083835b602083101515614dac5780518252602082019150602081019050602083039250614d87565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010189805190602001908083835b602083101515614e275780518252602082019150602081019050602083039250614e02565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010188805190602001908083835b602083101515614ea25780518252602082019150602081019050602083039250614e7d565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010187805190602001908083835b602083101515614f1d5780518252602082019150602081019050602083039250614ef8565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010186805190602001908083835b602083101515614f985780518252602082019150602081019050602083039250614f73565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010185805190602001908083835b6020831015156150135780518252602082019150602081019050602083039250614fee565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010184805190602001908083835b60208310151561508e5780518252602082019150602081019050602083039250615069565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010183805190602001908083835b60208310151561510957805182526020820191506020810190506020830392506150e4565b6001836020036101000a038019825116818451168082178552505050505050905001807f3b0000000000000000000000000000000000000000000000000000000000000081525060010182805190602001908083835b602083101515615184578051825260208201915060208101905060208303925061515f565b6001836020036101000a0380198251168184511680821785525050505050509050019b50505050505050505050505060405160208183030381529060405290505b919050565b6000600782608001516040518082805190602001908083835b60208310151561520857805182526020820191506020810190506020830392506151e3565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020905081600482600001604051808280546001816001161561010002031660029004801561529f5780601f1061527d57610100808354040283529182019161529f565b820191906000526020600020905b81548152906001019060200180831161528b575b505091505090815260200160405180910390208260010160405180828054600181600116156101000203166002900480156153115780601f106152ef576101008083540402835291820191615311565b820191906000526020600020905b8154815290600101906020018083116152fd575b505091505090815260200160405180910390208260020160405180828054600181600116156101000203166002900480156153835780601f10615361576101008083540402835291820191615383565b820191906000526020600020905b81548152906001019060200180831161536f575b5050915050908152602001604051809103902082600301548154811015156153a757fe5b90600052602060002090600c020160008201518160000190805190602001906153d19291906162af565b5060208201518160010190805190602001906153ee9291906162af565b50604082015181600201908051906020019061540b9291906162af565b5060608201518160030190805190602001906154289291906162af565b5060808201518160040190805190602001906154459291906162af565b5060a08201518160050190805190602001906154629291906162af565b5060c082015181600601908051906020019061547f9291906162af565b5060e082015181600701556101008201518160080155610120820151816009015561014082015181600a015561016082015181600b0160006101000a81548160ff0219169083151502179055509050505050565b60606040516020018060000190506040516020818303038152906040526040518082805190602001908083835b6020831015156155255780518252602082019150602081019050602083039250615500565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916846040516020018082805190602001908083835b60208310151561558f578051825260208201915060208101905060208303925061556a565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b6020831015156155f857805182526020820191506020810190506020830392506155d3565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916141561563757829050615751565b8382846040516020018084805190602001908083835b602083101515615672578051825260208201915060208101905060208303925061564d565b6001836020036101000a03801982511681845116808217855250505050505090500183805190602001908083835b6020831015156156c557805182526020820191506020810190506020830392506156a0565b6001836020036101000a03801982511681845116808217855250505050505090500182805190602001908083835b60208310151561571857805182526020820191506020810190506020830392506156f3565b6001836020036101000a038019825116818451168082178552505050505050905001935050505060405160208183030381529060405290505b9392505050565b60606008866040518082805190602001908083835b602083101515615792578051825260208201915060208101905060208303925061576d565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020805480602002602001604051908101604052809291908181526020016000905b828210156158a1578382906000526020600020018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561588d5780601f106158625761010080835404028352916020019161588d565b820191906000526020600020905b81548152906001019060200180831161587057829003601f168201915b5050505050815260200190600101906157e5565b50505050905060008151141561596a5760046040519080825280602002602001820160405280156158e657816020015b60608152602001906001900390816158d15790505b509050848160008151811015156158f957fe5b906020019060200201819052508381600181518110151561591657fe5b9060200190602002018190525061592c836160cb565b81600281518110151561593b57fe5b906020019060200201819052508181600381518110151561595857fe5b90602001906020020181905250615b43565b6159c281600081518110151561597c57fe5b90602001906020020151866040805190810160405280600181526020017f3b000000000000000000000000000000000000000000000000000000000000008152506154d3565b8160008151811015156159d157fe5b90602001906020020181905250615a368160018151811015156159f057fe5b90602001906020020151856040805190810160405280600181526020017f3b000000000000000000000000000000000000000000000000000000000000008152506154d3565b816001815181101515615a4557fe5b90602001906020020181905250615ab2816002815181101515615a6457fe5b90602001906020020151615a77856160cb565b6040805190810160405280600181526020017f3b000000000000000000000000000000000000000000000000000000000000008152506154d3565b816002815181101515615ac157fe5b90602001906020020181905250615b26816003815181101515615ae057fe5b90602001906020020151836040805190810160405280600181526020017f3b000000000000000000000000000000000000000000000000000000000000008152506154d3565b816003815181101515615b3557fe5b906020019060200201819052505b806008876040518082805190602001908083835b602083101515615b7c5780518252602082019150602081019050602083039250615b57565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390209080519060200190615bc29291906163af565b50505050505050565b6000806000600385600001516040518082805190602001908083835b602083101515615c0c5780518252602082019150602081019050602083039250615be7565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902085602001516040518082805190602001908083835b602083101515615c795780518252602082019150602081019050602083039250615c54565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020925084608001516040516020018082805190602001908083835b602083101515615ceb5780518252602082019150602081019050602083039250615cc6565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b602083101515615d545780518252602082019150602081019050602083039250615d2f565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390209150600090505b82805490508110156160c4578281815481101515615da257fe5b90600052602060002090600c02016004016040516020018082805460018160011615610100020316600290048015615e115780601f10615def576101008083540402835291820191615e11565b820191906000526020600020905b815481529060010190602001808311615dfd575b50509150506040516020818303038152906040526040518082805190602001908083835b602083101515615e5a5780518252602082019150602081019050602083039250615e35565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916826000191614156160b75760008414156160b25784600386600001516040518082805190602001908083835b602083101515615edc5780518252602082019150602081019050602083039250615eb7565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902086602001516040518082805190602001908083835b602083101515615f495780518252602082019150602081019050602083039250615f24565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902082815481101515615f8957fe5b90600052602060002090600c02016000820151816000019080519060200190615fb39291906162af565b506020820151816001019080519060200190615fd09291906162af565b506040820151816002019080519060200190615fed9291906162af565b50606082015181600301908051906020019061600a9291906162af565b5060808201518160040190805190602001906160279291906162af565b5060a08201518160050190805190602001906160449291906162af565b5060c08201518160060190805190602001906160619291906162af565b5060e082015181600701556101008201518160080155610120820151816009015561014082015181600a015561016082015181600b0160006101000a81548160ff0219169083151502179055509050505b6160c4565b8080600101915050615d88565b5050505050565b60606000806060600080861415616119576040805190810160405280600181526020017f30000000000000000000000000000000000000000000000000000000000000008152509450616219565b8593505b600084141515616143578280600101935050600a8481151561613b57fe5b04935061611d565b826040519080825280601f01601f1916602001820160405280156161765781602001602082028038833980820191505090505b5091506001830390505b60008614151561621557600a8681151561619657fe5b066030017f0100000000000000000000000000000000000000000000000000000000000000028282806001900393508151811015156161d157fe5b9060200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a905350600a8681151561620d57fe5b049550616180565b8194505b50505050919050565b6101806040519081016040528060608152602001606081526020016060815260200160608152602001606081526020016060815260200160608152602001600081526020016000815260200160008152602001600081526020016000151581525090565b608060405190810160405280606081526020016060815260200160608152602001600081525090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106162f057805160ff191683800117855561631e565b8280016001018555821561631e579182015b8281111561631d578251825591602001919060010190616302565b5b50905061632b919061640f565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061637057805160ff191683800117855561639e565b8280016001018555821561639e579182015b8281111561639d578251825591602001919060010190616382565b5b5090506163ab919061640f565b5090565b8280548282559060005260206000209081019282156163fe579160200282015b828111156163fd5782518290805190602001906163ed9291906162af565b50916020019190600101906163cf565b5b50905061640b9190616434565b5090565b61643191905b8082111561642d576000816000905550600101616415565b5090565b90565b61645d91905b8082111561645957600081816164509190616460565b5060010161643a565b5090565b90565b50805460018160011615610100020316600290046000825580601f1061648657506164a5565b601f0160209004906000526020600020908101906164a4919061640f565b5b505600a165627a7a72305820ea8e74f6c4dae0d20a8cfdcc1feffca5f6284359ddd82ff04282b8047528c7870029",
	// Permission master contract bytecode in genesis block, source code in kvm/smc/Permission.sol
	"0x00000000000000000000000000000000736D6336": "608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806302f0697b1461007d57806331e51ad414610174578063430ae633146101fb5780634665cb0714610278578063813775a3146102f5578063d1b7623c14610418575b600080fd5b34801561008957600080fd5b5061015e600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919080359060200190929190803590602001908201803590602001908080601f016020809104026020016040519081016040528093929190818152602001838380828437820191505050505050919291929050505061056b565b6040518082815260200191505060405180910390f35b34801561018057600080fd5b506101e5600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001909291905050506106dc565b6040518082815260200191505060405180910390f35b34801561020757600080fd5b50610262600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610811565b6040518082815260200191505060405180910390f35b34801561028457600080fd5b506102df600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610944565b6040518082815260200191505060405180910390f35b34801561030157600080fd5b5061035c600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610adf565b604051808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200184815260200183815260200180602001828103825283818151815260200191508051906020019080838360005b838110156103da5780820151818401526020810190506103bf565b50505050905090810190601f1680156104075780820380516001836020036101000a031916815260200191505b509550505050505060405180910390f35b34801561042457600080fd5b5061044360048036038101908080359060200190929190505050610e90565b60405180806020018673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200180602001858152602001848152602001838103835288818151815260200191508051906020019080838360005b838110156104c55780820151818401526020810190506104aa565b50505050905090810190601f1680156104f25780820380516001836020036101000a031916815260200191505b50838103825286818151815260200191508051906020019080838360005b8381101561052b578082015181840152602081019050610510565b50505050905090810190601f1680156105585780820380516001836020036101000a031916815260200191505b5097505050505050505060405180910390f35b60006001151561057a33610f74565b151514151561058857600080fd5b60a0604051908101604052808673ffffffffffffffffffffffffffffffffffffffff168152602001848152602001858152602001600115158152602001838152506000876040518082805190602001908083835b60208310151561060157805182526020820191506020810190506020830392506105dc565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550602082015181600101556040820151816002015560608201518160030160006101000a81548160ff02191690831515021790555060808201518160040190805190602001906106cb929190612ab5565b509050506001905095945050505050565b60006106e88383611012565b156106f6576001905061080b565b600015156000846040518082805190602001908083835b602083101515610732578051825260208201915060208101905060208303925061070d565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060030160009054906101000a900460ff1615151415610787576000905061080b565b816000846040518082805190602001908083835b6020831015156107c0578051825260208201915060208101905060208303925061079b565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600201541415610806576001905061080b565b600090505b92915050565b600061081c82611045565b1561082a576001905061093f565b600015156000836040518082805190602001908083835b6020831015156108665780518252602082019150602081019050602083039250610841565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060030160009054906101000a900460ff16151514156108bb576000905061093f565b600080836040518082805190602001908083835b6020831015156108f457805182526020820191506020810190506020830392506108cf565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060010154141561093a576000905061093f565b600190505b919050565b60006001151561095333610f74565b151514151561096157600080fd5b601061096c83611070565b101561097b5760009050610ada565b600015156000836040518082805190602001908083835b6020831015156109b75780518252602082019150602081019050602083039250610992565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060030160009054906101000a900460ff1615151415610a0c5760009050610ada565b6000826040518082805190602001908083835b602083101515610a445780518252602082019150602081019050602083039250610a1f565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600080820160006101000a81549073ffffffffffffffffffffffffffffffffffffffff0219169055600182016000905560028201600090556003820160006101000a81549060ff0219169055600482016000610ad39190612b35565b5050600190505b919050565b600080600060606000610af186611070565b90506010811015610b5457610b04611272565b81815181101515610b1157fe5b9060200190602002015160646001610b276118af565b84815181101515610b3457fe5b906020019060200201518292508191508090509450945094509450610e88565b600015156000876040518082805190602001908083835b602083101515610b905780518252602082019150602081019050602083039250610b6b565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060030160009054906101000a900460ff1615151415610c0857600080600082925081915080905060206040519081016040528060008152509450945094509450610e88565b6000866040518082805190602001908083835b602083101515610c405780518252602082019150602081019050602083039250610c1b565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902060000160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166000876040518082805190602001908083835b602083101515610ccf5780518252602082019150602081019050602083039250610caa565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600101546000886040518082805190602001908083835b602083101515610d3e5780518252602082019150602081019050602083039250610d19565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600201546000896040518082805190602001908083835b602083101515610dad5780518252602082019150602081019050602083039250610d88565b6001836020036101000a0380198251168184511680821785525050505050509050019150509081526020016040518091039020600401808054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610e785780601f10610e4d57610100808354040283529160200191610e78565b820191906000526020600020905b815481529060010190602001808311610e5b57829003601f168201915b5050505050905094509450945094505b509193509193565b6060600060606000806060601087101515610ee957600080600060206040519081016040528060008152509291908292506020604051908101604052806000815250919081915080905095509550955095509550610f6a565b610ef1611e11565b90508087815181101515610f0157fe5b90602001906020020151610f13611272565b88815181101515610f2057fe5b90602001906020020151610f326118af565b89815181101515610f3f57fe5b90602001906020020151610f51612aa3565b610f59612aac565b849450829250955095509550955095505b5091939590929450565b600060606000610f82611272565b9150600082511415610f97576000925061100b565b600090505b8151811015611006578373ffffffffffffffffffffffffffffffffffffffff168282815181101515610fca57fe5b9060200190602002015173ffffffffffffffffffffffffffffffffffffffff161415610ff9576001925061100b565b8080600101915050610f9c565b600092505b5050919050565b6000601061101f84611070565b10801561102c5750600182145b1561103a576001905061103f565b600090505b92915050565b60008061105183611070565b90506010811015611065576001915061106a565b600091505b50919050565b60006060600061107e611e11565b9150600090505b601081101561126657818181518110151561109c57fe5b906020019060200201516040516020018082805190602001908083835b6020831015156110de57805182526020820191506020810190506020830392506110b9565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b6020831015156111475780518252602082019150602081019050602083039250611122565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916846040516020018082805190602001908083835b6020831015156111b1578051825260208201915060208101905060208303925061118c565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b60208310151561121a57805182526020820191506020810190506020830392506111f5565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191614156112595780925061126b565b8080600101915050611085565b601092505b5050919050565b60608060106040519080825280602002602001820160405280156112a55781602001602082028038833980820191505090505b50905073c1fe56e3f58d3244f606306611a5d10c8333f1f68160008151811015156112cc57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff1681525050737cefc13b6e2aedeedfb7cb6c32457240746baee581600181518110151561132c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073ff3dac4f04ddbd24de5d6039f90596f0a8bb08fd81600281518110151561138c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073071e8f5ddddd9f2d4b4bdf8fc970dfe8d9871c288160038151811015156113ec57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250507394fd535aab6c01302147be7819d07817647f7b6381600481518110151561144c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073a8073c95521a6db54f4b5ca31a04773b093e92748160058151811015156114ac57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073e94517a4f6f45e80cbaaffbb0b845f4c0fdd754781600681518110151561150c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073ba30505351c17f4c818d94a990eded95e166474b81600781518110151561156c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073212a83c0d7db5c526303f873d9ceaa32382b55d08160088151811015156115cc57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff1681525050738db7cf1823fcfa6e9e2063f983b3b96a48eed5a481600981518110151561162c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250507366bab3f68ff0822b7ba568a58a5cb619c4825ce581600a81518110151561168c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250507388e1b4289b639c3b7b97899be32627dcd3e81b7e81600b8151811015156116ec57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073ce61e95666737e46b2453717fe1ba0d9a85b9d3e81600c81518110151561174c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff1681525050731a5193e85ffa06fde42b2a2a6da7535ba510ae8c81600d8151811015156117ac57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505073b19bc4477ff32ec13872a2a827782dea8b6e92c081600e81518110151561180c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff1681525050730fffa18f6c90ce3f02691dc5ec954495ea48304681600f81518110151561186c57fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250508091505090565b60608060106040519080825280602002602001820160405280156118e757816020015b60608152602001906001900390816118d25790505b5090506040805190810160405280600981526020017f5b3a3a5d3a33303030000000000000000000000000000000000000000000000081525081600081518110151561192f57fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303031000000000000000000000000000000000000000000000081525081600181518110151561198157fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a3330303200000000000000000000000000000000000000000000008152508160028151811015156119d357fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030330000000000000000000000000000000000000000000000815250816003815181101515611a2557fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030340000000000000000000000000000000000000000000000815250816004815181101515611a7757fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030350000000000000000000000000000000000000000000000815250816005815181101515611ac957fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030360000000000000000000000000000000000000000000000815250816006815181101515611b1b57fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030370000000000000000000000000000000000000000000000815250816007815181101515611b6d57fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030380000000000000000000000000000000000000000000000815250816008815181101515611bbf57fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a333030390000000000000000000000000000000000000000000000815250816009815181101515611c1157fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303130000000000000000000000000000000000000000000000081525081600a815181101515611c6357fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303131000000000000000000000000000000000000000000000081525081600b815181101515611cb557fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303132000000000000000000000000000000000000000000000081525081600c815181101515611d0757fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303133000000000000000000000000000000000000000000000081525081600d815181101515611d5957fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303134000000000000000000000000000000000000000000000081525081600e815181101515611dab57fe5b906020019060200201819052506040805190810160405280600981526020017f5b3a3a5d3a33303135000000000000000000000000000000000000000000000081525081600f815181101515611dfd57fe5b906020019060200201819052508091505090565b6060806010604051908082528060200260200182016040528015611e4957816020015b6060815260200190600190039081611e345790505b50905060a060405190810160405280608081526020017f376138366532623736323863373666636165373661386233373032356362613681526020017f393861323839613434313032633563303231353934623563396663653333303781526020017f326565376566393932663565303138646334346239386661313166656335333881526020017f3234643739303135373437653861633437346634656531356237666265383630815250816000815181101515611f0457fe5b9060200190602002018190525060a060405190810160405280608081526020017f363630383839653339623337616465353866373839393333393534313233653581526020017f366436343938393836613063643963613633643232336538363664353532316181526020017f616564633965353239386532663438323861356339306634633538666232346581526020017f3139363133613436326361303231306464393632383231373934663633306630815250816001815181101515611fc957fe5b9060200190602002018190525060a060405190810160405280608081526020017f326536316635373230316563383034663964353239386334363635383434666481526020017f303737613235313663643333656363656134386637626466393364653531383281526020017f646134663537646337623464383837306535653239316331373963303566663081526020017f343130303731386234393138346636346137633064343063633636333433646181525081600281518110151561208e57fe5b9060200190602002018190525060a060405190810160405280608081526020017f666334316137316437613734643836363564626363306634386339613630316581526020017f333062373134656435303634373636396566353263303366373132336632616581526020017f303738646361613336333839653236333665313035356635663630666466333881526020017f643839613232366565383432333466303036623333336361643264326263656581525081600381518110151561215357fe5b9060200190602002018190525060a060405190810160405280608081526020017f656266343666616361373534666339303731366436363565366336666562323081526020017f366361343337633965356631363639306536393035313362333032393335303581526020017f336139643732326238386432616230623937326634363434386633613533333781526020017f386266356366653031623833373361663265353431393762313736313765316381525081600481518110151561221857fe5b9060200190602002018190525060a060405190810160405280608081526020017f383063346662663635313232643831376433383038616663623638336663363681526020017f643966396531396234373665613065653366373537646361356364313833313681526020017f656362383939396266656134653961356163633939363835303463623931393981526020017f39376135633161623632336335633533336362363632323931313439623061338152508160058151811015156122dd57fe5b9060200190602002018190525060a060405190810160405280608081526020017f356437656438313331393136623130656135343561353539616265343634333081526020017f373130396133643632646462653139633336383938386262646231646432333381526020017f306236663362626234373964306264643739656633363064376439313735303081526020017f38643930663764353131323239363932313037393365386137353263656364368152508160068151811015156123a257fe5b9060200190602002018190525060a060405190810160405280608081526020017f376563643465613162663465666133346461633431613136643763636431346581526020017f323364333939336464336630613534643732326565373664313730373138616481526020017f626137663234366330383262616461393232633837356666616161343631386581526020017f333037623638653434633238343764346634653362373637383834633032623781525081600781518110151561246757fe5b9060200190602002018190525060a060405190810160405280608081526020017f343835376637393265663737396335313166366437363433663039393134303981526020017f663737653431313234636564313433383532313735333536343133313566356481526020017f633939323765373330316666643761666337616538303235363633653137663581526020017f393333303661646637643366666163376336616136323563323530646530643581525081600881518110151561252c57fe5b9060200190602002018190525060a060405190810160405280608081526020017f616436376332353032666332373233663264636632356131343037343433383281526020017f656233653465353064376534646439313063343233663761613466653066626281526020017f636332323037643232656636656466343639646436666265613733656661386481526020017f38376234623837366130643665333836633465303062366135316332613366388152508160098151811015156125f157fe5b9060200190602002018190525060a060405190810160405280608081526020017f343336393262366637323337306133326162396663353437376163336435363081526020017f653435643239646230653665646332653139356363373438343362386661646681526020017f333235396535373834633438626634303031613935366235376334653836386681526020017f366438643339323038333935333030396461313134653338326236376233323681525081600a8151811015156126b657fe5b9060200190602002018190525060a060405190810160405280608081526020017f653337363636346431376661353564316239343036316364343836613666386481526020017f363432636634336666363366323535353734346466363638633835663836653781526020017f383162303236643633303035363464646532333465313964643639643261626581526020017f646331386135336462306436663165346134633162663362323766376438343881525081600b81518110151561277b57fe5b9060200190602002018190525060a060405190810160405280608081526020017f326332396632636536346463393066313533386264653032663636363636346181526020017f366331343833323537363038366130313961613636373261666238653632393981526020017f303636623963353963386335313165393063303862373831653230353164653681526020017f666566626465363032646437343562356463383531393732663361333435373481525081600c81518110151561284057fe5b9060200190602002018190525060a060405190810160405280608081526020017f373939326636356638323339326331306531323762386138343266613738613181526020017f336239366134646434346331356563373738663037373363393162643661393381526020017f313663393738663231353566653433646535636431633165663961666463353381526020017f316464333362393839333031323364363961343634306632623663393531643581525081600d81518110151561290557fe5b9060200190602002018190525060a060405190810160405280608081526020017f323734396436633331333665393365303661363865386262626263386236393681526020017f626266623634343136366434653431353564336262646432373136353533633281526020017f613331376137383262663263376166366662386135313030323865646234326581526020017f363230343539373335633361333237323035333438616461643361396434316681525081600e8151811015156129ca57fe5b9060200190602002018190525060a060405190810160405280608081526020017f336230376566343839396233343631656164343263646464373232333165353881526020017f656663313133383038636630636132313136643937373239616632623230383781526020017f303661656535366238663130303161323036633032663034313762613132366281526020017f396365343937363466313663366536376534333865323438356338643662613481525081600f815181101515612a8f57fe5b906020019060200201819052508091505090565b60006064905090565b60006001905090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10612af657805160ff1916838001178555612b24565b82800160010185558215612b24579182015b82811115612b23578251825591602001919060010190612b08565b5b509050612b319190612b7d565b5090565b50805460018160011615610100020316600290046000825580601f10612b5b5750612b7a565b601f016020900490600052602060002090810190612b799190612b7d565b5b50565b612b9f91905b80821115612b9b576000816000905550600101612b83565b5090565b905600a165627a7a723058200ee9ea5a1b362a59cf948533b011d86f57700d6dcc44a8df40e5ef8a7a11d90f0029",
	// Candidate contract bytecode for private chain in genesis block, source code in kvm/smc/permissioned/CandidateDB.sol
	// This contract should be moved to another config for private chain soon
	"0x00000000000000000000000000000000736d6337": "60806040526004361061008e576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063126d89ff146100935780631a83260c1461012357806332955d7014610218578063536bbec3146103315780638813ce1214610459578063b9da4af8146104e9578063cc5ef00914610598578063f406872214610628575b600080fd5b34801561009f57600080fd5b506100a861071d565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156100e85780820151818401526020810190506100cd565b50505050905090810190601f1680156101155780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561012f57600080fd5b50610216600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610bac565b005b34801561022457600080fd5b5061031560048036038101908080359060200190929190803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610d27565b604051808260ff1660ff16815260200191505060405180910390f35b34801561033d57600080fd5b506103de600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f016020809104026020016040519081016040528093929190818152602001838380828437820191505050505050919291929050505061136d565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561041e578082015181840152602081019050610403565b50505050905090810190601f16801561044b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561046557600080fd5b5061046e61180f565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156104ae578082015181840152602081019050610493565b50505050905090810190601f1680156104db5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156104f557600080fd5b50610596600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050611c9e565b005b3480156105a457600080fd5b506105ad611dab565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156105ed5780820151818401526020810190506105d2565b50505050905090810190601f16801561061a5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561063457600080fd5b5061071b600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050612230565b005b60608060008060008054905014156107475760206040519081016040528060008152509250610ba7565b60206040519081016040528060008152509150600090505b600080549050811015610ba3576001151560008281548110151561077f57fe5b906000526020600020906006020160030160009054906101000a900460ff1615151415610b96576040516020018060000190506040516020818303038152906040526040518082805190602001908083835b6020831015156107f657805182526020820191506020810190506020830392506107d1565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916826040516020018082805190602001908083835b602083101515610860578051825260208201915060208101905060208303925061083b565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b6020831015156108c957805182526020820191506020810190506020830392506108a4565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191614151561094357610940826040805190810160405280600181526020017f2c0000000000000000000000000000000000000000000000000000000000000081525061231d565b91505b610b9382610b8e60008481548110151561095957fe5b906000526020600020906006020160c06040519081016040529081600082018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610a0c5780601f106109e157610100808354040283529160200191610a0c565b820191906000526020600020905b8154815290600101906020018083116109ef57829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610aae5780601f10610a8357610100808354040283529160200191610aae565b820191906000526020600020905b815481529060010190602001808311610a9157829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610b505780601f10610b2557610100808354040283529160200191610b50565b820191906000526020600020905b815481529060010190602001808311610b3357829003601f168201915b505050505081526020016003820160009054906101000a900460ff1615151515815260200160048201548152602001600582015481525050846123e9565b61231d565b91505b808060010191505061075f565b8192505b505090565b7fe2df4c83eef1ab3c88f888395f79598f4f2b295eb4701a77c0278a2027392b3f83838360405180806020018060200180602001848103845287818151815260200191508051906020019080838360005b83811015610c18578082015181840152602081019050610bfd565b50505050905090810190601f168015610c455780820380516001836020036101000a031916815260200191505b50848103835286818151815260200191508051906020019080838360005b83811015610c7e578082015181840152602081019050610c63565b50505050905090810190601f168015610cab5780820380516001836020036101000a031916815260200191505b50848103825285818151815260200191508051906020019080838360005b83811015610ce4578082015181840152602081019050610cc9565b50505050905090810190601f168015610d115780820380516001836020036101000a031916815260200191505b50965050505050505060405180910390a1505050565b6000836040516020018082805190602001908083835b602083101515610d625780518252602082019150602081019050602083039250610d3d565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b602083101515610dcb5780518252602082019150602081019050602083039250610da6565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916600086815481101515610e0b57fe5b90600052602060002090600602016000016040516020018082805460018160011615610100020316600290048015610e7a5780601f10610e58576101008083540402835291820191610e7a565b820191906000526020600020905b815481529060010190602001808311610e66575b50509150506040516020818303038152906040526040518082805190602001908083835b602083101515610ec35780518252602082019150602081019050602083039250610e9e565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916141515610f045760009050611365565b600085815481101515610f1357fe5b906000526020600020906006020160030160009054906101000a900460ff1615610f405760009050611365565b816040516020018082805190602001908083835b602083101515610f795780518252602082019150602081019050602083039250610f54565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b602083101515610fe25780518252602082019150602081019050602083039250610fbd565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191660008681548110151561102257fe5b906000526020600020906006020160010160405160200180828054600181600116156101000203166002900480156110915780601f1061106f576101008083540402835291820191611091565b820191906000526020600020905b81548152906001019060200180831161107d575b50509150506040516020818303038152906040526040518082805190602001908083835b6020831015156110da57805182526020820191506020810190506020830392506110b5565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191614151561111b5760009050611365565b600160008681548110151561112c57fe5b906000526020600020906006020160030160006101000a81548160ff0219169083151502179055508260008681548110151561116457fe5b90600052602060002090600602016002019080519060200190611188929190612cf3565b504360008681548110151561119957fe5b9060005260206000209060060201600501819055507f8621b9013be815a7c03fe9292e5009d853e9bfc6985fb0415eb3d92cde37f19a84846000888154811015156111e057fe5b906000526020600020906006020160010160405180806020018060200180602001848103845287818151815260200191508051906020019080838360005b8381101561123957808201518184015260208101905061121e565b50505050905090810190601f1680156112665780820380516001836020036101000a031916815260200191505b50848103835286818151815260200191508051906020019080838360005b8381101561129f578082015181840152602081019050611284565b50505050905090810190601f1680156112cc5780820380516001836020036101000a031916815260200191505b5084810382528581815460018160011615610100020316600290048152602001915080546001816001161561010002031660029004801561134e5780601f106113235761010080835404028352916020019161134e565b820191906000526020600020905b81548152906001019060200180831161133157829003601f168201915b5050965050505050505060405180910390a1600190505b949350505050565b606060008090505b6001805490508110156117f457836040516020018082805190602001908083835b6020831015156113bb5780518252602082019150602081019050602083039250611396565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b60208310151561142457805182526020820191506020810190506020830392506113ff565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191660018281548110151561146457fe5b906000526020600020906005020160000160405160200180828054600181600116156101000203166002900480156114d35780601f106114b15761010080835404028352918201916114d3565b820191906000526020600020905b8154815290600101906020018083116114bf575b50509150506040516020818303038152906040526040518082805190602001908083835b60208310151561151c57805182526020820191506020810190506020830392506114f7565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040518091039020600019161480156117225750826040516020018082805190602001908083835b60208310151561158e5780518252602082019150602081019050602083039250611569565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b6020831015156115f757805182526020820191506020810190506020830392506115d2565b6001836020036101000a03801982511681845116808217855250505050505090500191505060405180910390206000191660018281548110151561163757fe5b906000526020600020906005020160010160405160200180828054600181600116156101000203166002900480156116a65780601f106116845761010080835404028352918201916116a6565b820191906000526020600020905b815481529060010190602001808311611692575b50509150506040516020818303038152906040526040518082805190602001908083835b6020831015156116ef57805182526020820191506020810190506020830392506116ca565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916145b156117e75760018181548110151561173657fe5b90600052602060002090600502016002018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156117db5780601f106117b0576101008083540402835291602001916117db565b820191906000526020600020905b8154815290600101906020018083116117be57829003601f168201915b50505050509150611808565b8080600101915050611375565b602060405190810160405280600081525091505b5092915050565b60608060008060008054905014156118395760206040519081016040528060008152509250611c99565b60206040519081016040528060008152509150600090505b600080549050811015611c95576040516020018060000190506040516020818303038152906040526040518082805190602001908083835b6020831015156118ae5780518252602082019150602081019050602083039250611889565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916826040516020018082805190602001908083835b60208310151561191857805182526020820191506020810190506020830392506118f3565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b602083101515611981578051825260208201915060208101905060208303925061195c565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040518091039020600019161415156119fb576119f8826040805190810160405280600181526020017f2c0000000000000000000000000000000000000000000000000000000000000081525061231d565b91505b60001515600082815481101515611a0e57fe5b906000526020600020906006020160030160009054906101000a900460ff1615151415611c8857611c8582611c80600084815481101515611a4b57fe5b906000526020600020906006020160c06040519081016040529081600082018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015611afe5780601f10611ad357610100808354040283529160200191611afe565b820191906000526020600020905b815481529060010190602001808311611ae157829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015611ba05780601f10611b7557610100808354040283529160200191611ba0565b820191906000526020600020905b815481529060010190602001808311611b8357829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015611c425780601f10611c1757610100808354040283529160200191611c42565b820191906000526020600020905b815481529060010190602001808311611c2557829003601f168201915b505050505081526020016003820160009054906101000a900460ff1615151515815260200160048201548152602001600582015481525050846123e9565b61231d565b91505b8080600101915050611851565b8192505b505090565b611ca6612d73565b60c0604051908101604052808481526020018381526020016020604051908101604052806000815250815260200160001515815260200143815260200160008152509050600081908060018154018082558091505090600182039060005260206000209060060201600090919290919091506000820151816000019080519060200190611d34929190612dac565b506020820151816001019080519060200190611d51929190612dac565b506040820151816002019080519060200190611d6e929190612dac565b5060608201518160030160006101000a81548160ff0219169083151502179055506080820151816004015560a08201518160050155505050505050565b6060806000806001805490501415611dd5576020604051908101604052806000815250925061222b565b60206040519081016040528060008152509150600090505b600180549050811015612227576040516020018060000190506040516020818303038152906040526040518082805190602001908083835b602083101515611e4a5780518252602082019150602081019050602083039250611e25565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916826040516020018082805190602001908083835b602083101515611eb45780518252602082019150602081019050602083039250611e8f565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040526040518082805190602001908083835b602083101515611f1d5780518252602082019150602081019050602083039250611ef8565b6001836020036101000a038019825116818451168082178552505050505050905001915050604051809103902060001916141515611f9757611f94826040805190810160405280600181526020017f2c0000000000000000000000000000000000000000000000000000000000000081525061231d565b91505b60011515600182815481101515611faa57fe5b906000526020600020906005020160030160009054906101000a900460ff161515141561221a5761221782612212600184815481101515611fe757fe5b906000526020600020906005020160a06040519081016040529081600082018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561209a5780601f1061206f5761010080835404028352916020019161209a565b820191906000526020600020905b81548152906001019060200180831161207d57829003601f168201915b50505050508152602001600182018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561213c5780601f106121115761010080835404028352916020019161213c565b820191906000526020600020905b81548152906001019060200180831161211f57829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156121de5780601f106121b3576101008083540402835291602001916121de565b820191906000526020600020905b8154815290600101906020018083116121c157829003601f168201915b505050505081526020016003820160009054906101000a900460ff161515151581526020016004820154815250508461290c565b61231d565b91505b8080600101915050611ded565b8192505b505090565b612238612e2c565b60a0604051908101604052808581526020018481526020018381526020016001151581526020014381525090506001819080600181540180825580915050906001820390600052602060002090600502016000909192909190915060008201518160000190805190602001906122af929190612dac565b5060208201518160010190805190602001906122cc929190612dac565b5060408201518160020190805190602001906122e9929190612dac565b5060608201518160030160006101000a81548160ff0219169083151502179055506080820151816004015550505050505050565b606082826040516020018083805190602001908083835b6020831015156123595780518252602082019150602081019050602083039250612334565b6001836020036101000a03801982511681845116808217855250505050505090500182805190602001908083835b6020831015156123ac5780518252602082019150602081019050602083039250612387565b6001836020036101000a03801982511681845116808217855250505050505090500192505050604051602081830303815290604052905092915050565b6060806123f583612b9c565b905083606001511561270a578084600001518560200151866040015161241e8860800151612b9c565b61242b8960a00151612b9c565b6040516020018087805190602001908083835b602083101515612463578051825260208201915060208101905060208303925061243e565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010186805190602001908083835b6020831015156124de57805182526020820191506020810190506020830392506124b9565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010185805190602001908083835b6020831015156125595780518252602082019150602081019050602083039250612534565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010184805190602001908083835b6020831015156125d457805182526020820191506020810190506020830392506125af565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010183805190602001908083835b60208310151561264f578051825260208201915060208101905060208303925061262a565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010182805190602001908083835b6020831015156126ca57805182526020820191506020810190506020830392506126a5565b6001836020036101000a03801982511681845116808217855250505050505090500196505050505050506040516020818303038152906040529150612905565b80846000015185602001516127228760800151612b9c565b6040516020018085805190602001908083835b60208310151561275a5780518252602082019150602081019050602083039250612735565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010184805190602001908083835b6020831015156127d557805182526020820191506020810190506020830392506127b0565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010183805190602001908083835b602083101515612850578051825260208201915060208101905060208303925061282b565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010182805190602001908083835b6020831015156128cb57805182526020820191506020810190506020830392506128a6565b6001836020036101000a03801982511681845116808217855250505050505090500194505050505060405160208183030381529060405291505b5092915050565b60608061291883612b9c565b9050808460000151856020015186604001516129378860800151612b9c565b6040516020018086805190602001908083835b60208310151561296f578051825260208201915060208101905060208303925061294a565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010185805190602001908083835b6020831015156129ea57805182526020820191506020810190506020830392506129c5565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010184805190602001908083835b602083101515612a655780518252602082019150602081019050602083039250612a40565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010183805190602001908083835b602083101515612ae05780518252602082019150602081019050602083039250612abb565b6001836020036101000a038019825116818451168082178552505050505050905001807f3a0000000000000000000000000000000000000000000000000000000000000081525060010182805190602001908083835b602083101515612b5b5780518252602082019150602081019050602083039250612b36565b6001836020036101000a0380198251168184511680821785525050505050509050019550505050505060405160208183030381529060405291505092915050565b60606000806060600080861415612bea576040805190810160405280600181526020017f30000000000000000000000000000000000000000000000000000000000000008152509450612cea565b8593505b600084141515612c14578280600101935050600a84811515612c0c57fe5b049350612bee565b826040519080825280601f01601f191660200182016040528015612c475781602001602082028038833980820191505090505b5091506001830390505b600086141515612ce657600a86811515612c6757fe5b066030017f010000000000000000000000000000000000000000000000000000000000000002828280600190039350815181101515612ca257fe5b9060200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a905350600a86811515612cde57fe5b049550612c51565b8194505b50505050919050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10612d3457805160ff1916838001178555612d62565b82800160010185558215612d62579182015b82811115612d61578251825591602001919060010190612d46565b5b509050612d6f9190612e5e565b5090565b60c06040519081016040528060608152602001606081526020016060815260200160001515815260200160008152602001600081525090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10612ded57805160ff1916838001178555612e1b565b82800160010185558215612e1b579182015b82811115612e1a578251825591602001919060010190612dff565b5b509050612e289190612e5e565b5090565b60a060405190810160405280606081526020016060815260200160608152602001600015158152602001600081525090565b612e8091905b80821115612e7c576000816000905550600101612e64565b5090565b905600a165627a7a72305820e1c4d65e1920e9ecdbd117be10475145dbc886cbdac679dc7567918b26201a890029",
	// Candidate exchange contract bytecode for Kardia-private chain dual node, source code in kvm/smc/permissioned/CandidateExchange.sol
	"0x00000000000000000000000000000000736d6338": "60806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630e40683614610051578063912991d314610146575b600080fd5b34801561005d57600080fd5b50610144600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610281565b005b34801561015257600080fd5b5061027f600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506103fc565b005b7f3ca643a7086eb63dfb0e5f2cec44808b9487badc9a643e9eaae2415149fb833c83838360405180806020018060200180602001848103845287818151815260200191508051906020019080838360005b838110156102ed5780820151818401526020810190506102d2565b50505050905090810190601f16801561031a5780820380516001836020036101000a031916815260200191505b50848103835286818151815260200191508051906020019080838360005b83811015610353578082015181840152602081019050610338565b50505050905090810190601f1680156103805780820380516001836020036101000a031916815260200191505b50848103825285818151815260200191508051906020019080838360005b838110156103b957808201518184015260208101905061039e565b50505050905090810190601f1680156103e65780820380516001836020036101000a031916815260200191505b50965050505050505060405180910390a1505050565b7f90affc9ed2543eb1fb9de02387ab117d255429f9f5c25458d725cc772bc7221f848484846040518080602001806020018060200180602001858103855289818151815260200191508051906020019080838360005b8381101561046d578082015181840152602081019050610452565b50505050905090810190601f16801561049a5780820380516001836020036101000a031916815260200191505b50858103845288818151815260200191508051906020019080838360005b838110156104d35780820151818401526020810190506104b8565b50505050905090810190601f1680156105005780820380516001836020036101000a031916815260200191505b50858103835287818151815260200191508051906020019080838360005b8381101561053957808201518184015260208101905061051e565b50505050905090810190601f1680156105665780820380516001836020036101000a031916815260200191505b50858103825286818151815260200191508051906020019080838360005b8381101561059f578082015181840152602081019050610584565b50505050905090810190601f1680156105cc5780820380516001836020036101000a031916815260200191505b509850505050505050505060405180910390a1505050505600a165627a7a72305820aaf69639545c4279771ad00169e14a25b6d1d929396974fc4d1fa08cf2af26440029",
}

// abi for contract in genesis block
var GenesisContractAbis = map[string]string{
	// This is abi for counter contract
	"0x00000000000000000000000000000000736d6331": `[
		{"constant": false,"inputs": [{"name": "x","type": "uint8"}],"name": "set","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": true,"inputs": [],"name": "get","outputs": [{"name": "","type": "uint8"}],"payable": false,"stateMutability": "view","type": "function"}
	]`,
	// This is abi for simple voting contract
	"0x00000000000000000000000000000000736d6332": `[
		{"constant": true,"inputs": [{"name": "toProposal","type": "uint8"}],"name": "getVote","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": true,"inputs": [],"name": "winningProposal","outputs": [{"name": "_winningProposal","type": "uint8"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": false,"inputs": [{"name": "toProposal","type": "uint8"}],"name": "vote","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"}
	]`,
	// This is abi for master exchange contract
	"0x00000000000000000000000000000000736d6333": `[
		{"constant":true,"inputs":[],"name":"getNeoToSend","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},
		{"constant":false,"inputs":[{"name":"neo","type":"uint256"}],"name":"removeNeo","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
		{"constant":true,"inputs":[],"name":"totalEth","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},
		{"constant":false,"inputs":[{"name":"eth","type":"uint256"}],"name":"removeEth","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
		{"constant":true,"inputs":[],"name":"totalNeo","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},
		{"constant":false,"inputs":[{"name":"neo","type":"uint256"}],"name":"matchNeo","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
		{"constant":false,"inputs":[{"name":"eth","type":"uint256"}],"name":"matchEth","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
		{"constant":true,"inputs":[],"name":"getEthToSend","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}
	]`,
	"0x00000000000000000000000000000000736D6339": `[
	{
		"constant": false,
		"inputs": [
			{
				"name": "fromType",
				"type": "string"
			},
			{
				"name": "toType",
				"type": "string"
			},
			{
				"name": "fromAddress",
				"type": "string"
			},
			{
				"name": "receiver",
				"type": "string"
			},
			{
				"name": "txid",
				"type": "string"
			},
			{
				"name": "amount",
				"type": "uint256"
			},
			{
				"name": "timestamp",
				"type": "uint256"
			}
		],
		"name": "addOrder",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [],
		"name": "setOwner",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "fromType",
				"type": "string"
			},
			{
				"name": "toType",
				"type": "string"
			},
			{
				"name": "fromAmount",
				"type": "uint256"
			},
			{
				"name": "receivedAmount",
				"type": "uint256"
			}
		],
		"name": "updateRate",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "txid",
				"type": "string"
			}
		],
		"name": "getMatchingResult",
		"outputs": [
			{
				"name": "results",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "user",
				"type": "address"
			}
		],
		"name": "deAuthorizedUser",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "_type",
				"type": "string"
			}
		],
		"name": "getAddressFromType",
		"outputs": [
			{
				"name": "",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "_address",
				"type": "string"
			},
			{
				"name": "fromType",
				"type": "string"
			},
			{
				"name": "toType",
				"type": "string"
			}
		],
		"name": "getAllOrders",
		"outputs": [
			{
				"name": "orders",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getOwner",
		"outputs": [
			{
				"name": "",
				"type": "address"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "fromType",
				"type": "string"
			},
			{
				"name": "toType",
				"type": "string"
			}
		],
		"name": "getPendingOrders",
		"outputs": [
			{
				"name": "orders",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "user",
				"type": "address"
			}
		],
		"name": "authorizedUser",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "txId",
				"type": "string"
			}
		],
		"name": "getOrderByTxIdPublic",
		"outputs": [
			{
				"name": "strOrder",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "fromType",
				"type": "string"
			},
			{
				"name": "toType",
				"type": "string"
			}
		],
		"name": "getRate",
		"outputs": [
			{
				"name": "fromAmount",
				"type": "uint256"
			},
			{
				"name": "receivedAmount",
				"type": "uint256"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "txId",
				"type": "string"
			}
		],
		"name": "hasTxId",
		"outputs": [
			{
				"name": "",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "txId",
				"type": "string"
			},
			{
				"name": "kardiaTxId",
				"type": "string"
			}
		],
		"name": "updateKardiaTx",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "txId",
				"type": "string"
			},
			{
				"name": "targetTxId",
				"type": "string"
			}
		],
		"name": "updateTargetTx",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "_type",
				"type": "string"
			},
			{
				"name": "_address",
				"type": "string"
			}
		],
		"name": "updateAvailableType",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "user",
				"type": "address"
			}
		],
		"name": "isAuthorized",
		"outputs": [
			{
				"name": "",
				"type": "bool"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`,
	"0x00000000000000000000000000000000736D6336": `[
		{"constant": true,"inputs": [{"name": "_pubkey","type": "string"},{"name": "_nodeType","type": "uint256"}],
	"name": "isValidNode","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "view",
	"type": "function"},
		{"constant": true,"inputs": [{"name": "_pubkey","type": "string"}],"name": "isValidator","outputs": [{"name": "",
	"type": "uint256"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": true,"inputs": [{"name": "_pubkey","type": "string"}],"name": "getNodeInfo","outputs": [{"name": "addr",
	"type": "address"},{"name": "votingPower","type": "uint256"},{"name": "nodeType","type": "uint256"}, {"name": "listenAddress","type": "string"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": false,"inputs": [{"name": "_pubkey","type": "string"},{"name": "_addr","type": "address"},{"name": "_nodeType",
	"type": "uint256"},{"name": "_votingPower","type": "uint256"},{"name": "listenAddress","type": "string"}],"name": "addNode","outputs": [{"name": "","type": "uint256"}],
	"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": false,"inputs": [{"name": "_pubkey","type": "string"}],"name": "removeNode","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "nonpayable","type": "function"
	},
		{"constant": true,"inputs": [{"name": "index","type": "uint256"}],"name": "getInitialNodeByIndex","outputs": [{"name": "publickey","type": "string"},{"name": "addr","type": "address"},{"name": "listenAddr","type": "string"},{"name": "votingPower","type": "uint256"},{"name": "nodeType","type": "uint256"}],"payable": false,"stateMutability": "pure","type": "function"}
	]`,
	// Abi for private chain candidate contract to manage candidate data
	"0x00000000000000000000000000000000736D6337": `[
		{"constant": true,"inputs": [],"name": "getCompletedRequests","outputs": [{"name": "requestList","type": "string"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": false,"inputs": [{"name": "_email","type": "string"},{"name": "_fromOrgId","type": "string"},{"name": "_toOrgId","type": "string"}],"name": "requestCandidateInfo","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": false,"inputs": [{"name": "_requestID","type": "uint256"},{"name": "_email","type": "string"},{"name": "_content","type": "string"},{"name": "_toOrgId","type": "string"}],"name": "completeRequest","outputs": [{"name": "","type": "uint8"}],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": true,"inputs": [{"name": "_email","type": "string"},{"name": "_fromOrgId","type": "string"}],"name": "getExternalResponse","outputs": [{"name": "content","type": "string"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": true,"inputs": [],"name": "getRequests","outputs": [{"name": "requestList","type": "string"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": false,"inputs": [{"name": "email","type": "string"},{"name": "fromOrgId","type": "string"}],"name": "addRequest","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": true,"inputs": [],"name": "getResponses","outputs": [{"name": "responseList","type": "string"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": false,"inputs": [{"name": "_email","type": "string"},{"name": "_fromOrgId","type": "string"},{"name": "_content","type": "string"}],"name": "addExternalResponse","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"anonymous": false,"inputs": [{"indexed": false,"name": "email","type": "string"},{"indexed": false,"name": "fromOrgId","type": "string"},{"indexed": false,"name": "toOrgId","type": "string"}],"name": "ExternalCandidateInfoRequested","type": "event"},
		{"anonymous": false,"inputs": [{"indexed": false,"name": "email","type": "string"},{"indexed": false,"name": "answer","type": "string"},{"indexed": false,"name": "toOrgId","type": "string"}],"name": "RequestCompleted","type": "event"}]`,
	// Abi for Kardia contract to exchange candidate data between private chain
	"0x00000000000000000000000000000000736D6338": `[
		{"constant": false,"inputs": [{"name": "_email","type": "string"},{"name": "_fromOrgID","type": "string"},{"name": "_toOrgID","type": "string"}],"name": "forwardRequest","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": false,"inputs": [{"name": "_email","type": "string"},{"name": "_response","type": "string"},{"name": "_fromOrgID","type": "string"},{"name": "_toOrgID","type": "string"}],"name": "forwardResponse","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"anonymous": false,"inputs": [{"indexed": false,"name": "email","type": "string"},{"indexed": false,"name": "fromOrgID","type": "string"},{"indexed": false,"name": "toOrgID","type": "string"}],"name": "IncomingRequest","type": "event"},
		{"anonymous": false,"inputs": [{"indexed": false,"name": "email","type": "string"},{"indexed": false,"name": "name","type": "string"},{"indexed": false,"name": "age","type": "uint8"},{"indexed": false,"name": "addr","type": "address"},{"indexed": false,"name": "source","type": "string"},{"indexed": false,"name": "fromOrgID","type": "string"},{"indexed": false,"name": "toOrgID","type": "string"}],"name": "FulfilledRequest","type": "event"}]`,
}

func GetContractAddressAt(index int) common.Address {
	if index >= len(GenesisContractAddress) {
		return common.Address{}
	}
	return common.HexToAddress(GenesisContractAddress[index])
}

func GetContractAbiByAddress(address string) string {
	// log.Info("Getting abi for address",  "address", address)
	for add, abi := range GenesisContractAbis {
		if strings.EqualFold(add, address) {
			return abi
		}
	}
	panic("abi not found")
}

func GetContractDetailsByIndex(index int) (common.Address, string) {
	address := GetContractAddressAt(index)
	if address.Equal(common.Address{}) {
		return address, ""
	}
	return address, GetContractAbiByAddress(address.String())
}

func GetRateFromType(chain string) *big.Int {
	switch chain {
	case NEO: return big.NewInt(RateNEO)
	case TRON: return big.NewInt(RateTRON)
	case ETH: return big.NewInt(RateETH)
	default: return nil
	}
}
