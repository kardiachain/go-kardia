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
	"0x8A93d2Aa374187a34BC03C679116d046599b6D3D": InitValueInCell, "0x24bdaA15F8D8C7889d2c66f6b8eb116eb084baf9": InitValueInCell, "0xF038bE927215E63DfE371894e818Ec96d2E51Ed3": InitValueInCell, "0x74036b7Fd50f9D4a257c0cED896Be48aFac9b535": InitValueInCell, "0x17d99DA7ec2a181677Ca2114A09Bff20C73EDb5f": InitValueInCell, "0x2214F1028Ae6C816dbB772d1333B7Cc801075a6c": InitValueInCell, "0x4Cb633Ee13B0f1Adeb5B34B2856B3e94F31619c5": InitValueInCell, "0x49E306Ef7d6bC201F54fdB4307AC79F01BfE5623": InitValueInCell, "0xF3b30813B73e35fe2068a3eFd6C3207e5f8b190d": InitValueInCell, "0x015803741Ec982b7bC8Ee658A98eB044d5c948C7": InitValueInCell, "0xf8b4FA2558aB6A881813f911Ba09E41885a87E6a": InitValueInCell, "0x99eeF578CAB154051Cc57F35945d1CC805c6f94A": InitValueInCell, "0x5af9Fa9DBB707A4Bb0d90A37F8b09E03a855d0a7": InitValueInCell, "0x6a61e9Ff463AC20520dd8cC41B9D87C7CE562200": InitValueInCell, "0x9F71cf60066d5124ef311a78acc6b25aD5098f06": InitValueInCell, "0xB65a35CD011447e5ec6FA547a44Bc8220Ad096f0": InitValueInCell,

	"0x6a0dA650FFa028938Db0b2e2e22Fe5Aa16f23e0c": InitValueInCell,
	"0x3159DA22E6E2F89ad0496d5EfE0d509314f43Efc": InitValueInCell,
	"0x8EAE4f71D6aB0bFf530C128c8b38433baDf5FD14": InitValueInCell,
	"0x476801B6278b79C8e7Beb93f00839308908e03a6": InitValueInCell,
	"0x59917EA5FD174257d71aB50a69833363eFF2A837": InitValueInCell,
	"0x5721B24BFf1E3a4E4a5B025abFb56bDd45f48C10": InitValueInCell,
	"0xFfc15c19205BB49EbEDb339848a27bdA16cD9D3D": InitValueInCell,
	"0x2e9312626d73eFCcF469712C95BaE56602F7BcD5": InitValueInCell,
	"0xb2749995A879e9ef4772213f5e8ED046c31F7CCA": InitValueInCell,
	"0xCe03fff3128105f2038839d37350883BD656a6D9": InitValueInCell,
	"0xC5C7158072151D5C3FdB9DA156BB6e761eCa6821": InitValueInCell,
	"0x054F85769C1f87804797932e6F3bC80755a01AEC": InitValueInCell,
	"0xC543881B2492c79Bc11BAdDC2cd05b49b7edAB7F": InitValueInCell,
	"0x562109F8209755EE394D1aB6Da87d9f819C6A2F9": InitValueInCell,
	"0x75700De48851B9D4460f455926cB1F4dc0D0c5B8": InitValueInCell,
	"0xa7e372f9E7AAe8336A4BFf33082fe954c7Ef47D8": InitValueInCell,
	"0x6dd344Ff484AdeE94934623e1F0A32F576F17Cf1": InitValueInCell,
	"0x00Ee6C2EFe0f06aD5F2440244cA902065633C19b": InitValueInCell,
	"0x1a57EAE941e55DC219Cd456352c9d90035d4d41A": InitValueInCell,
	"0xa1fc5571BA8344E96C72bCdC792e0E9fe205E6A2": InitValueInCell,
	"0x8d9A9cCd185b10C40e27A84bd4Fe8ccDE26d35FF": InitValueInCell,
	"0x37E1Fe38858ECDb2685DB2F17499E7Dd4f189529": InitValueInCell,
	"0x00B1c424736af61384447A681001BDEd424Ff7a2": InitValueInCell,
	"0x39450Ba610522aAaE1d002AA033Fd432F33757d0": InitValueInCell,
	"0xeC7DA1c4268d04200A4D0Cc9aa7e80c719a8FB4d": InitValueInCell,
	"0xE709E4425B1047bC732Cbf6D4C759EFeE784A99C": InitValueInCell,
	"0xBeB6Bd33AC1b4146aaF946021F15363949031D53": InitValueInCell,
	"0x7681229737d98811c22bEc685dCA77a5E3935D48": InitValueInCell,
	"0xF7bdD3358832cFBF2D7D16dCCfC35BF1d64f786F": InitValueInCell,
	"0x17E6e7E1Af1Da51Bd443D23C4624816356097FC7": InitValueInCell,
	"0xeE701975bFD65C7d0A89412DC38F8be1baEFdb85": InitValueInCell,
	"0xfDEb7AfBCa2771DE3eddbFCFc7e70Ecc33A18957": InitValueInCell,
	"0x09378a640AfB04126fb1817a7CfC03D265b3B8Cb": InitValueInCell,
	"0xB195C652313F2D9df50b03BB239beFe63c440324": InitValueInCell,
	"0x0031E3C30Eda319bf6790a83Dff14006d859ce48": InitValueInCell,
	"0x9fBaf00672913CC057E371520d902A7ee0F2e25b": InitValueInCell,
	"0x192b39199fe8445DF8D1e263B647A1A0489D0f73": InitValueInCell,
	"0x9701703f81b4188EfDFc277685400439aBf3F793": InitValueInCell,
	"0x91405A9D6067420c6448755bADB31e28977d940d": InitValueInCell,
	"0x3e19C4A0D5c72a70351FbcF363E1127A4e4D9489": InitValueInCell,
	"0x88155c923F30Bee02ecE40a7112f43a63fd79Ac0": InitValueInCell,
	"0xbF296a6135252Ca2626710b7662753b91E86f136": InitValueInCell,
	"0x176eb379C47704C6ca109316F3e20De2F8379f17": InitValueInCell,
	"0x12c9ccb7Cc1a502027069053674ebaFe461B74E5": InitValueInCell,
	"0xAb5Fcc4fc96aCBf846750CA89aD37D9949c14199": InitValueInCell,
	"0xE72Be2b990d9284CAf7d97A5dE5Aca6088D409f8": InitValueInCell,
	"0x29f2c85318E9AAFB97B5A1077d03abAC249EFdC7": InitValueInCell,
	"0xf5d8A073F3B12fBCdeE83dAe0704d4B318f42dBD": InitValueInCell,
	"0xF45251DEF21171BDC495ECB01214C8B54b8bbC5A": InitValueInCell,
	"0x6E26Fb2A1F7F2741DDC3a7588a3827b0602247D7": InitValueInCell,
	"0x4f36A53DC32272b97Ae5FF511387E2741D727bdb": InitValueInCell,
	"0x3013d14272220C4654ad2623CA4bB6D65Eed3858": InitValueInCell,
	"0x4ACfAd4c4E92395f81252DcB5cC08e09ED21BD26": InitValueInCell,
	"0xf07A33Bd5982d974ad14AB01b63148359Da1518D": InitValueInCell,
	"0x347bf6D78F87c3cC268C1FE794D841C3AE5Eb3A8": InitValueInCell,
	"0x659f985c71260b0917B376D33c91984Ac0C14B3f": InitValueInCell,
	"0xD8815f1FFec9ed357cb23E6e49D8b4CBEE982918": InitValueInCell,
	"0x154b9a9D6C16b236901548f218a6cF6223a8D297": InitValueInCell,
	"0xd16FfD214942b78F7F7ac601eD4D1C30C84490Ec": InitValueInCell,
	"0xB897e32114258a6b821F1F7ACCF5931240222A11": InitValueInCell,
	"0x8535C9D584f00185b2Dd1800d4852847bB85D6bF": InitValueInCell,
	"0xdEd895A776a189F978C85B24EB3fF8d5b310fEd8": InitValueInCell,
	"0xF2dF88c8398cDCc1cc6610a51a7b713be19428Be": InitValueInCell,
	"0x35dDc8986ca45BDB8C755982cFaFF031634EcCB8": InitValueInCell,
	"0x5B4cd1209b51A366364E514E68b1f2Fc581937Bd": InitValueInCell,
	"0xf6dB4C5fB3bC858714338d55e009698a667f2aF0": InitValueInCell,
	"0x469f8AF52EcB4a9d4E358232F7ECd10a199777d1": InitValueInCell,
	"0x8F151FC5b854C976Bb70B154CCc13FD14dF1d3a5": InitValueInCell,
	"0x9765CBD085E692C00b81abC1a064E4A15777960b": InitValueInCell,
	"0xb107A8E037d2D0fa01169c1fF991e6E6D2C17Be0": InitValueInCell,
	"0x1b20541F248D53CA403445c630C6D619C811926b": InitValueInCell,
	"0x2c0603848c16B88A36420EdE7aDb78c9523380fD": InitValueInCell,
	"0x9Cd3487a19680EF6F01F9e02c10c6339A9412ABa": InitValueInCell,
	"0xF751b5986764918aB24ceEF919681A159CF1C492": InitValueInCell,
	"0x98db63a89Fe8c54d7f8DDCc00D1A6Ff5554137e0": InitValueInCell,
	"0x30EEE9F6E9Be9a55eDFC7765aB80b866bfab3387": InitValueInCell,
	"0x2b9e2CF96E7F7676df1Df451C6Fd807C5f12e729": InitValueInCell,
	"0x82A59Cb0182E0C0BA89eD2b1961E67b50805E742": InitValueInCell,
	"0xDEA515C9CEcF292C39a8f4c7f185191674E5e23d": InitValueInCell,
	"0xea31feB6EDe2a1F9E15a6196BFa1B3b01700896D": InitValueInCell,
	"0xF93D95fbBCEcC3Aaa0E2e1AB33CA78df67b9D663": InitValueInCell,
	"0xeb68cb2ED3D70463922d4b78699B4c73142F73A4": InitValueInCell,
	"0x03628904AB1b784Fb3cd00359ef40faBED8C055F": InitValueInCell,
	"0x3BF1aFa1E387A9060C3fA9Ef2920B758F2Eb8AcB": InitValueInCell,
	"0x393Fe03a0868c268DCaEcEF596b1D86745D3Ef47": InitValueInCell,
	"0x6D16cf8290D4826c2e23970cCa43F2FF19F0D36e": InitValueInCell,
	"0x430A827EAe164adb1984a44F953c13772699AAf5": InitValueInCell,
	"0x084B1A57978Ff4D8D85a426bA695aC3E1C4Bc876": InitValueInCell,
	"0xd87265C8Ce2D2463293f571218dEA3D685feb396": InitValueInCell,
	"0xCE4d69809506777477937C288f6Edd5f7F963483": InitValueInCell,
	"0xaBB1A6ea4b23D1a4FB3072DddA5e7Ad6088871CC": InitValueInCell,
	"0xa88fb954C857CBB49605e9B69226a1BB77968F8D": InitValueInCell,
	"0x34128D82Becb2BC19bd00ef8202Af2cD3343Ca18": InitValueInCell,
	"0x273a4Da86FEDC61A762FCd459e37D035aCeb2E9A": InitValueInCell,
	"0x85daaa16D41cC06b3AeAB2633ED5945fdE1170f2": InitValueInCell,
	"0xbd04Fb13b26E4cc102cC3e87A97809e6D970bcdc": InitValueInCell,
	"0xE499C3Af337860EbA7c326d55eC5515aD03Df236": InitValueInCell,
	"0x3F6f726E742ceE1d4762b48ef8448DBdFdAa3996": InitValueInCell,
	"0x517c3622D06f1942096a633D941d4CbEDDf4D881": InitValueInCell,
	"0xD6391141744939684422361750DB1Edf37276f81": InitValueInCell,
	"0x3670d4E841A0AdC6E08Aa6F40E430D5fbfE390Fe": InitValueInCell,
	"0x1347Fa7F4a7048Df396233a289a6AF0403C1F952": InitValueInCell,
	"0xC0a872C458A1DE6ddC7027aBE2522C280edf6e6F": InitValueInCell,
	"0xfE33Cee33041654C2BFed4FBE9bB1C43eb0eb3FD": InitValueInCell,
	"0xe167bC277257030C7B5B032888F13cbb06b924e0": InitValueInCell,
	"0x8d62D733393C20A789CED16a45F047115A8F6699": InitValueInCell,
	"0x671FFf5fBCc577e610b25484553511206fD13f8b": InitValueInCell,
	"0x4Bd0AafD1780C3Ec6903b17fd3B5A28507A21568": InitValueInCell,
	"0x817EE72bb1B1b730D9A50CB1388b2E8eB3dA27b1": InitValueInCell,
	"0x0a2bb5665E3cD584852C1484aa4085a91880a4C1": InitValueInCell,
	"0x658C6436Dd43D029cD2d7625029B887A19f27e71": InitValueInCell,
	"0x57c81659247C8192C959cf211FE4576bDdDCB207": InitValueInCell,
	"0xd65d66379a6B588e11935b6bc3895d1485d6e982": InitValueInCell,
	"0x456175Ae8e2139E13Bd1F90f42A7715cBeFEa3F7": InitValueInCell,
	"0x61B03e0d450Be094f81071B7a696424F4A4f3Da5": InitValueInCell,
	"0xeB904b9CF3C4B3d336e8415b79E099c1e38F0938": InitValueInCell,
	"0x4468b6d0288695c04Ba8eDC25971c355711B4b0f": InitValueInCell,
	"0x8391E9DA8f76424aF748550af419dFD3EB48C547": InitValueInCell,
	"0x5EB0ee09220C6d20Eb339C1A0332A0aF65E88c8f": InitValueInCell,
	"0x66dB9a93E95D13f3fc129047643d572b3E9F5BC5": InitValueInCell,
	"0xA3D996d84d158F8455b2f8F75311464715504Ac9": InitValueInCell,
	"0x8D82416d8E93869fDA2E3c4A6f662d4Ca110D85b": InitValueInCell,
	"0x0beA13364Bb2177dc4b0Be1421F48e362D26Ce40": InitValueInCell,
	"0xc647B36073B7b77C539386353C4f4a20bB5c9Bc4": InitValueInCell,
	"0xEacEb177F268a8f57f441e596a800F6aE1F0caA4": InitValueInCell,
	"0x8272ef99c6c28B1732df6980db620C829D7C5650": InitValueInCell,
	"0x32453F63480196Df9875830b952c7cE0d2C6f040": InitValueInCell,
	"0x8860E7450ab5795C60B09b05c9c0aef909ffC16d": InitValueInCell,
	"0xe6d05064F30c6D28905c5fF9393f380d7fC6e72C": InitValueInCell,
	"0x735024Ec4ac939B4baB6983E9CcEeC90700F8cA7": InitValueInCell,
	"0x9FF37978CD4EC0720ECD8D0C8eF79296407cbD21": InitValueInCell,
	"0x7fFA5A9f7d0FFd09762d6849BCFC5b8FDEFD630A": InitValueInCell,
	"0x5A8a70884c83548122f0D2c8718A9aF8BC657477": InitValueInCell,
	"0xA730B4DeEDb36A896F7D77Ff18195941FDa1E066": InitValueInCell,
	"0x92CcBdd5B0a2fE4950DD99B448D32EC7EbEad8F3": InitValueInCell,
	"0x343DEBF01fd6143B48C3DFe5330A26169Fdc67E3": InitValueInCell,
	"0xeED66e966448417e96a3c58A74F1851423b8f626": InitValueInCell,
	"0x527C3d6bBaf5551d8767b81EAf7BFF5a50A209DB": InitValueInCell,
	"0x885D40651Ce820a8459E4C4a9Bf0c7630413E0F2": InitValueInCell,
	"0x1996ec73BDb9eB2348A95F3a4664C336bF651bA9": InitValueInCell,
	"0xDff7E2F72E67158eA64Ce836F194Fb64593F6630": InitValueInCell,
	"0x5fe3672235335fD93689220b4ea7CD7Ad34dE3fC": InitValueInCell,
	"0xBf0C90C7eCBA6C517e85d9ebc74F48482ab3ace6": InitValueInCell,
	"0x1f2c8C3193C2b1b7C2f92b83c4c346806586e105": InitValueInCell,
	"0xf314322CBFba5794cC88812f07d79E0ACe5bB762": InitValueInCell,
	"0xe94eE2bdDF0b9a23412F8e3BCa08b508a711017b": InitValueInCell,
	"0x18274E21310DEa944f3079d2aeA12101655848d5": InitValueInCell,
	"0xEB5C6758a3c19245f2c99cac4471e613185a2ac2": InitValueInCell,
	"0x95FFd8F3bAEb30215d7bCaFe5DEf423AB9454ad9": InitValueInCell,
	"0x6372a0E02078d57bf81e01FC5deC45eaD366AADe": InitValueInCell,
	"0xA07A5774F8Ebd2C786efb367dfddfee5e6CDC53A": InitValueInCell,
	"0xa35d8c3d07e2097b422bCD7589146123B66835C6": InitValueInCell,
	"0x331Cd21d7Ef4e1FcA028e492a2Dac66706d30a50": InitValueInCell,
	"0x2B598b6731245E9d23CA780205A26604e3C999a2": InitValueInCell,
	"0xd8E682438a84B391491734C75Bdc104BEA2a0bB5": InitValueInCell,
	"0xf49bEeEdCdCa43f35AbD1059f12454eb7c43e007": InitValueInCell,
	"0x53617B38405174518DbdAbC3A650b39735daC735": InitValueInCell,
	"0x9F37720e7Ea8103Ef31EE9eB14c5bef51a619512": InitValueInCell,
	"0x27d8d6aaF8F099B549b4047DF9ba35DD002fB59e": InitValueInCell,
	"0xfEFd8a7EBB93335594a6e59a63f283358e92D1ca": InitValueInCell,
	"0x21039d05771BF4f8BAa8C497540ACbD35D6Cc9d2": InitValueInCell,
	"0xa85Df4Af0c3d4CA56222Da042F9c07940d5b2727": InitValueInCell,
	"0x5816fd102cE4c87C9d6E36b4B97DA1AD9e09593D": InitValueInCell,
	"0x1CF86F387A3DaE2868AB2cEB5306c269A58c0b3d": InitValueInCell,
	"0xE3BD9570Bb1287138FB02FF4aeC9C057e1BD7551": InitValueInCell,
	"0xE1234156D3e5A420d2A552723a31a915c2c5966e": InitValueInCell,
	"0x16B8c53112F76947Bb057A4563f565ABC0a301da": InitValueInCell,
	"0xcac727aa38DBdE3467C8E946cb183b7Ad8cc04Ee": InitValueInCell,
	"0x5079f1E28336F4C3B1a91FC8C8553C0f8aEeFfC7": InitValueInCell,
	"0xaD3203923e5993A20c30c976D67f79255677eC96": InitValueInCell,
	"0xC23E56d4C3eB526756a0d7DAC4C45085134e79cC": InitValueInCell,
	"0x4B6A760Af1CBeC59cf9B513225BF2d1ddA4e18Fd": InitValueInCell,
	"0x9659778Fa650411DA6e8987c316c26a484190518": InitValueInCell,
	"0x5BC44895b7480181d9BB9E9d8671038477BfDfb3": InitValueInCell,
	"0x772e2E63cd80f50F95cff38986cB10170CC96e41": InitValueInCell,
	"0x4A6f6085ab4AAbcaAaDCDcb5d5D6816209F99F3d": InitValueInCell,
	"0x221EE91E5DB13bB170BB654EB357D8D1b47e62c2": InitValueInCell,
	"0xB34a8AdC476E671Bd45A8375F54BC3Ac6729D146": InitValueInCell,
	"0x177A299C364bbA3148E0180770601427B34C4279": InitValueInCell,
	"0x92660Bd3721AF3812E3f4692bF477054a2903a7E": InitValueInCell,
	"0x1Bff6Fb01a1057B9D53E57da7437dAE755d2B966": InitValueInCell,
	"0xc9e3249E8D47167E4287a175C20C0e9F9d80b934": InitValueInCell,
	"0xD824bebf8b8565542a7495Be1D4F5A38a731a91F": InitValueInCell,
	"0x0b7767B4a96de1d95605d27594519dE32b054018": InitValueInCell,
	"0xA65c160F0765D9aaCED686972ed7c98Bb135491a": InitValueInCell,
	"0xB27aae37278C934A77d7fbd39f3aD11d0C1C2ccA": InitValueInCell,
	"0xEf77C2F1F797Dfb42cF2D6388Ea6197386d98CF8": InitValueInCell,
	"0x35353CB9f05CED2569777bEa15d29CCF8A19EC47": InitValueInCell,
	"0xA4762Ef75BB581E9cB51BC849Ec365542c719438": InitValueInCell,
	"0x5A8EF098F9A6c883dd6421250bf9Dc91B587385B": InitValueInCell,
	"0x7F7911556f0Fa0472e9582cb2B7877773F9C3B19": InitValueInCell,
	"0xb88C1800AedC31966a292dc2d921ec3Db2522862": InitValueInCell,
	"0xe5D8480e529D132c6d55bd442A304b6244c0dbB6": InitValueInCell,
	"0x88e6B9EB555d1C34C8D4d090CCF1071dBd8688F8": InitValueInCell,
	"0x18Ba59A8B9154806b4bD55daeFEbF09998b8FfE4": InitValueInCell,
	"0x5897029b2EEaAE2D971848Dfb9851459B0d74193": InitValueInCell,
	"0x27243418429e0847D4575e8E9f7B61F5c9A09fa7": InitValueInCell,
	"0x4e781cc6e04cE76225991C0C7b7fd0892fc62955": InitValueInCell,
	"0x7a99269a00A8E4dDCC43bedF19389565A2fF0d91": InitValueInCell,
	"0xaC89b0d5D6b3Ea76072aAe079A037367fd01DD1c": InitValueInCell,
	"0x5AB759881d027561Fce51B422deE6A48713f4d80": InitValueInCell,
	"0xb23155D0CF067874D3206AE47429869F40BAcD81": InitValueInCell,
	"0xb4Ee24dEd407c36D7dE988Df1556B43B9ffC167c": InitValueInCell,
	"0xfEb5b171249905aDD90eA9ab10865C72601D54aD": InitValueInCell,
	"0xB050F1609E34E40321B64b18a202EB606DbA41a0": InitValueInCell,
	"0xB5dF1b692c37799f775f247c0387826710a2aEcE": InitValueInCell,
	"0x0d8f13df4E7bD39f206E6b49b252b6e43C0FdE87": InitValueInCell,
	"0x9890E838967F3946F54524AAEaFAa862D69FE515": InitValueInCell,
	"0xDBEc981FD4220c380B24a8E7FD3Ee47F79949325": InitValueInCell,
	"0xe47cF6bb6f943147c03f9200cddE61c7Df707dBe": InitValueInCell,
	"0x351Ad913B02f0Fe95757d47c1d206F1bbC57C0Cb": InitValueInCell,
	"0x8d044A5E890B9F842376B55bEA12e4A6eB67db59": InitValueInCell,
	"0xAEDA02e080335F1B6Ce1313D1b5Ba1fC79ee977e": InitValueInCell,
	"0xdca6A02D1B0C93578bd0aFC4eEd1D97B1221F862": InitValueInCell,
	"0x840c2fe44b88af27CeBE2598C8f8371c3735b59E": InitValueInCell,
	"0xaf135d99d0624206A9957096c8e00C7ac9501F07": InitValueInCell,
	"0xec9CF62dDC5EC8b145aE10Bb0e07874e490Be123": InitValueInCell,
	"0xb90735bE9dFEcC8101f17B6D2E8509B0a49A0496": InitValueInCell,
	"0x25CE013F42839Bb09BdBEBBAfD88d33da4a6FbE7": InitValueInCell,
	"0xd57BeDE413f41E73121EAdF9F150EaF3776F7fF2": InitValueInCell,
	"0x9958f8aE195C35fFCD35E7321Db602fc2B35995d": InitValueInCell,
	"0xcb17544CB20393E8e85A0F6f667c4c82e6346c6e": InitValueInCell,
	"0x78bf32Ce0b6C0a8D634E933182D6A40A4b584943": InitValueInCell,
	"0xB95f9E6Ae1b69b8c96CB53bD5487E8aF17991eB1": InitValueInCell,
	"0xEabFD008Ed71E3FF7c861dB4cEbD54F5C2B6f381": InitValueInCell,
	"0xeD007FcfD6398C9Ea4B04a9F5FDc5F3CF5Bf2394": InitValueInCell,
	"0xeF1F611d871Dc539E7831CC494ed4d556E15634B": InitValueInCell,
	"0xd70fF30a6fbF2D968D05a44D758961dD29c5cA8D": InitValueInCell,
	"0x0474C30aB5525553B8Db6eA0B3Da24eC355E027E": InitValueInCell,
	"0x73769E89a7b8cD0fE3aa6AE54F32CA0aFD16b5DB": InitValueInCell,
	"0x1558381335D70c2B97fDf76bDA1caaE5Bc8dA767": InitValueInCell,
	"0x884CF7D856b7572d52200A816FB07a213e5E729d": InitValueInCell,
	"0x418957DB47CaC2936832Ea712C12433f05E44552": InitValueInCell,
	"0x78E794Fd5AaE9d76AE97AA04B05352fAFaE9C397": InitValueInCell,
	"0x0826cd0B2E2111e24158D8c3D5556c9e4b8037F1": InitValueInCell,
	"0xF93dC6002227D124D6d037d4F3392c18c04f21e4": InitValueInCell,
	"0x9D7Caa289428c4C690EC6144822368A20e984067": InitValueInCell,
	"0x22D5Ced4C05F04687CB5b3FEdF18079BB3636592": InitValueInCell,
	"0x50dee5466CEAc1e1b880a315C224d62041c6ffD8": InitValueInCell,
	"0xb0956d4ef6CCADBc75c83234826E5ceA24d41568": InitValueInCell,
	"0xfa5eDF2d1D51B1c140e537f3f11e32232dB34e98": InitValueInCell,
	"0xdC9148b6e4E23A46dD76A8bBCd9029d244a4Eb4a": InitValueInCell,
	"0x748019daa742DEC4034009D9E368ccE45b6efd6D": InitValueInCell,
	"0xA2c8992Ba761B69e0B1e3cCA686200D7685d9229": InitValueInCell,
	"0xb961E01570BDF2Ac2dE9Cd7518bC027ce28C4932": InitValueInCell,
	"0x8E4D802Ca42AaCE90123a5CE69DD18Ab18BFE966": InitValueInCell,
	"0x1093AD441C85F861118Ad0230A2D2DF51555dd0E": InitValueInCell,
	"0xFfe2566580F08b6E42B1Cd713c1887158153DfE7": InitValueInCell,
	"0xa36d45ecDaf8f3A922532b9ca7DFDC428c9357FA": InitValueInCell,
	"0xEe727280f574fc81FaF05D153CcEFf4af1D08e83": InitValueInCell,
	"0x04b057136F3a1e1131434592E5d6209891726984": InitValueInCell,
	"0x45dD0B611Cacb95dEBE7be429b872D92953c7301": InitValueInCell,
	"0xF8b8cEe574dcfAa87Bf356C9994E2f7087517165": InitValueInCell,
	"0x6c32b5Ff6cc20113767e18b69A6d23B2965E3Dc6": InitValueInCell,
	"0xD2D94924a6D99f58CEBA21470101aa2537a9167a": InitValueInCell,
	"0x19Bd5140df2411F6485033d6F33602B6200671dB": InitValueInCell,
	"0x9cA23b2af9Fa928F56E1375c09Ec7d0CcbD8617f": InitValueInCell,
	"0xB7074f96497973ba2b3D598c24bE3d568fA1e3C0": InitValueInCell,
	"0x14A45C9120A3A0a8B32d34bb109624658917b71C": InitValueInCell,
	"0x1e2069B8F864F3db3878e799a3F40Df9aad6e9e6": InitValueInCell,
	"0x795C1Ea3c24496fFa41Ada71411d6CC1dbe327D1": InitValueInCell,
	"0x67603Af26D11E351f9B1BdbD34800969BDB505b8": InitValueInCell,
	"0x302dFB4a3E79Fb5e52308E57d95B69824141F2b8": InitValueInCell,
	"0x59660638Bb4FdAD4062E6329AA9b7CdE11b01Db4": InitValueInCell,
	"0xf25128F38C10fD96868de05A4f4c5668141452b0": InitValueInCell,
	"0xE20aDB0AfE4296977868aE7F54f93F96b48D2de2": InitValueInCell,
	"0x24D9D2e946b8905977d707A5A482Ac41d929C57c": InitValueInCell,
	"0x85f488313dDD3F0b23F6F64CDcC08eC044b16B9E": InitValueInCell,
	"0xbEF8B2c7fB2b1eC8dE91aD525C0B32f43bdC5FaC": InitValueInCell,
	"0x3c7d65C29e0E31cb77E5F018ab9402B38FAa36d2": InitValueInCell,
	"0xd2Df380C8648add1Bdadc3884dc234e918070De4": InitValueInCell,
	"0x1FD53eF5E5F22aD745245f3B16541AEc60cBD42e": InitValueInCell,
	"0xfB8Dd31052B658e36A53050662a40c5a26cbe8f1": InitValueInCell,
	"0x82471E29EFD8fd6D7328e6366e8EF6240BF48D2D": InitValueInCell,
	"0xa2F84326C9a8cc2e4918b2d062fe358Ace222edf": InitValueInCell,
	"0x114B4718aeB64209262bc2b75c4Cc88Ef7ca6885": InitValueInCell,
	"0xa9b690725Bf6a4Bc834f8fa32D7dB4D254b11419": InitValueInCell,
	"0xbC8E41D821Da6d9532131c5858E9b79aC195b618": InitValueInCell,
	"0x6De82deCB01041a91337F2A2BfEE4BBFf7C2F960": InitValueInCell,
	"0xC28537992B07fFd4B27B182cE984065552F3e078": InitValueInCell,
	"0x2dc01055524438143Ca84b90b5CF66CC4B1D2a14": InitValueInCell,
	"0x82aD02241549e79681991FE366178F1285757d0C": InitValueInCell,
	"0x9ef64d8E076E16441819aB9818651e060f1968E5": InitValueInCell,
	"0x75E097BB62EF2C601d0095a92A5Fd48e120d41C2": InitValueInCell,
	"0x8fE3F91b8963a6C4D128B4eEbcf63D8A83A02d80": InitValueInCell,
	"0x1744467d9385EC219B95cCd0Ea9034225457540c": InitValueInCell,
	"0xbCA43b9044649E8f181cA07452E950a148348E15": InitValueInCell,
	"0x8B060991cF5FD334AdcD4eAbd94595Bb6f08Cf28": InitValueInCell,
	"0xc52dcb6dF8C5125706F966F2f10664Ce123b0486": InitValueInCell,
	"0x38F64E3915aC3aeb8a0bCE9899A73D338801b8D9": InitValueInCell,
	"0xaCdb0A8A95393B54Bbc1bBc4a1FC35C8BD003e4a": InitValueInCell,
	"0xdF0c449bB6EdD6aCE64201bCd437d8767d2Eb8D8": InitValueInCell,
	"0x7087252fbC35578628F7cdF065132F6f3e0f0eFE": InitValueInCell,
	"0x081f36794e56a0f8Af21a34323b1eEB561db46A9": InitValueInCell,
	"0xad02ab314894DB437C2eaB581e71eafa7A2179f2": InitValueInCell,
	"0x42655333489a5DFE6251dAc994Fecb5a1700a8Ef": InitValueInCell,
	"0xFF448DdF7E5C003A8572C4E561c06b1D13b5c1db": InitValueInCell,
	"0x216F58b2B721B002F726Cf6702dFe4d06e152989": InitValueInCell,
	"0xB68C77c5b335543DF4B9099B094ceDCba1848BD5": InitValueInCell,
	"0x961631d0c17211aFDd6d7A58ec331D812656cf7A": InitValueInCell,
	"0x9e6F688059c8583B0699075f20E5774a865Ad669": InitValueInCell,
	"0xcdD89551A82101d69A20fD267d00eD8b7b918A5d": InitValueInCell,
	"0x80EcBB528f70cD607Cfa94865aFBA50b4c36cDfB": InitValueInCell,
	"0x81D245E903748F87eD8A0FA0f786D0859b4a8208": InitValueInCell,
	"0xeA12e5B40e3C733DdB13b73147B4B58b4D1080A0": InitValueInCell,
	"0xaD0bfB99473606DA7513f8BaE58ECFa47bb1752e": InitValueInCell,
	"0x0e8ea75DE08A428ABaD52a4e738A6023DFae29cA": InitValueInCell,
	"0x3746d8B917081019Be70c0A19cef8E8Ac344B37f": InitValueInCell,
	"0x163923a6A51a8c78D5623C84Ce303AeB8C7C016a": InitValueInCell,
	"0x9A0DBe2A02039EE25545Ef029A4DBb0e890D6278": InitValueInCell,
	"0x09720D4ce1eD9BF59AE9Af4EC6A41f87DE3fe2c5": InitValueInCell,
	"0x04e5174bdc99713E1EB24d0Dd648Fe7372743a0D": InitValueInCell,
	"0x4d6C9FFEBcdD3f34D42537887D7fC81c4a3f5052": InitValueInCell,
	"0x9F4bDF10291597b2AD2B0dCFFd90834C8227df0f": InitValueInCell,
	"0xAA8564b772e6357bf4016e573aA409DBCFbF9e68": InitValueInCell,
	"0x085d50BB4a54C018Dc39efcD6c626f390716856D": InitValueInCell,
	"0xeF24085A18CE48f2A31B3250b837E129c13c645e": InitValueInCell,
	"0xf4Efa66048dD4339cE3c8bB7767C687711e47be6": InitValueInCell,
	"0x1299eA3C6894b8f8b9bc881E5D2c078bDd2E827E": InitValueInCell,
	"0x867902A8134011A32dcc9B00553A76Bea408BdED": InitValueInCell,
	"0xBDA1F3Cab11F45d971b32064b5091fc78142ED17": InitValueInCell,
	"0xDe19f4944716757aF2C9B63598685Cc3Cd4e1F17": InitValueInCell,
	"0xDdd3764D26c74fce90CeB5b9E250aAB9D73cb327": InitValueInCell,
	"0xbC24c185Ae574c3d55eE8Cf08B67874A22d71Ef8": InitValueInCell,
	"0x9d988F8665d13F6E79a7390f34A35FF06e7A0CfA": InitValueInCell,
	"0xa7ff5138DbD052c702DAa3F637317ff023Ae0197": InitValueInCell,
	"0x3aB9b44Dbf41c11529a2B60772A53e2026581769": InitValueInCell,
	"0x82F0463D8728d9928587cD46c4BE225aABe810dB": InitValueInCell,
	"0x9a141b57C8107755Bab955d1803D886EB8C25494": InitValueInCell,
	"0x8735FA5555Fe70127DE9e9C3b68333506B8F5D26": InitValueInCell,
	"0x6a547246886f6A75fdB3558f19B03Ca9e4c11506": InitValueInCell,
	"0xBC0C5a3b81e739F638Be76a07e21984e93f2989F": InitValueInCell,
	"0xde0918C68EcE228adb6bCfc0B1280FFE0DF9e58B": InitValueInCell,
	"0x9f8641398c017c586D76d69b55DD478B81f41Dc8": InitValueInCell,
	"0x3f5be4E4E5D9C9fA0011A5b8661cF9AC26D592d5": InitValueInCell,
	"0x119b888b00BAa5013d649491e65F60203F24af8e": InitValueInCell,
	"0xbAbe3BdF1E1CB5be5015D0d29963B58c85Ae730b": InitValueInCell,
	"0xf2aE11E787269a8D7a4553A0CcB91865bb505C6a": InitValueInCell,
	"0x914bB6E5F5FadfE9577544199037129822983912": InitValueInCell,
	"0x8b79E9a62A2cB9fe8a37eA4E1A01c61bBD78aA73": InitValueInCell,
	"0x26f27316452E6d10D7C37A53cB09Df242F04162E": InitValueInCell,
	"0x20aEd0b9f2371149ceDE8Bb565e5b5f26E25E45e": InitValueInCell,
	"0x1C594631Ad3b97b9a111e76d46CCA521A0E47f66": InitValueInCell,
	"0xE7677bd67007E77b49C66C6A0a6bEe75C4177184": InitValueInCell,
	"0x0EC75412B817435b1E7F5D6bA223a7B3ce88d349": InitValueInCell,
	"0x3783496C758d9C9a97d9118F597B08d822D9d67E": InitValueInCell,
	"0xa0EfC699Db0c226e9a8eAF11D27Ca1d7Bc815854": InitValueInCell,
	"0xCd7b57D1ce9E9f3D5a38Eaa4960803B3eDAE489F": InitValueInCell,
	"0x886906c1BF89bD5a5265bc3fccC9C4E053F52050": InitValueInCell,
	"0x7b2A8573243cF7e7F85da21753919DC3d9f38659": InitValueInCell,
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
	"0x6a0dA650FFa028938Db0b2e2e22Fe5Aa16f23e0c": "96a5fd9fe8d1255a72d69c12e4a7c062e50d5f73f52810b559d1f2875c0a2937",
	"0x3159DA22E6E2F89ad0496d5EfE0d509314f43Efc": "ed29fab0534f76e4c23edc6f42325d6dd81b71a908fe3c42cbf5bbb14b1728e5",
	"0x8EAE4f71D6aB0bFf530C128c8b38433baDf5FD14": "c21deef6ea0dc043ac5af8ca9b75716f8c20de35721a4fae8aa51aa715f00134",
	"0x476801B6278b79C8e7Beb93f00839308908e03a6": "897b6b3dc7f360f8c4646f16c25744098a034f95e9b7e2bfb924918a1920a796",
	"0x59917EA5FD174257d71aB50a69833363eFF2A837": "d6c662bc7f21f8b4f993525f09ad9b2b735618c6c5576fc600d867ca5b096c21",
	"0x5721B24BFf1E3a4E4a5B025abFb56bDd45f48C10": "2c3505f17a49b671aaa4a77a98408f5bccb208788ff9d631e0538d287f4aac70",
	"0xFfc15c19205BB49EbEDb339848a27bdA16cD9D3D": "3d7a1a1bc97150c87741bf28b43d3e61762c208c2f2f63fbbb3ca204794a775f",
	"0x2e9312626d73eFCcF469712C95BaE56602F7BcD5": "6313b0bde01de760a330c4b7fbe6515dfd839fc4ed71f836f54ecfa89aed929a",
	"0xb2749995A879e9ef4772213f5e8ED046c31F7CCA": "cfb46c0929fdfd3f21695edede37d8a72ce62d7ef6dd98433b54c3b9391d746b",
	"0xCe03fff3128105f2038839d37350883BD656a6D9": "ee7d454c6cebd837502dcd877488a660301f8e02d154ef2b589dfeb6566cc047",
	"0xC5C7158072151D5C3FdB9DA156BB6e761eCa6821": "b9f234c884281334e7bff788de95f65a726daed53a55c14902379f0f4c66ca03",
	"0x054F85769C1f87804797932e6F3bC80755a01AEC": "9a52abe58efb9aff6f069a92807a98bea164393b9e2dd57fcfde815bfe7ef80a",
	"0xC543881B2492c79Bc11BAdDC2cd05b49b7edAB7F": "d5fe217d1e9083fe806f63da07568de690961768d6051be542fe70cbaa448d22",
	"0x562109F8209755EE394D1aB6Da87d9f819C6A2F9": "e14630a0cca17fe106dfaf451624ae2d236f1aab914f2873c21f7177c00dedf6",
	"0x75700De48851B9D4460f455926cB1F4dc0D0c5B8": "f17a0eedb25c72b197fa02070792b65d1befa604f5fd2f64edbf23e9c8f7118d",
	"0xa7e372f9E7AAe8336A4BFf33082fe954c7Ef47D8": "9b2f9d8a3a4d831f97b0ad88a952b29b5953c561ced59a1f5a4c45f95a4811ba",
	"0x6dd344Ff484AdeE94934623e1F0A32F576F17Cf1": "265f5ef67270638505174941100202b4e0257c4330c422293d7c849ab3ff8187",
	"0x00Ee6C2EFe0f06aD5F2440244cA902065633C19b": "ad29a85927fd063aaa495f959b8ca235675ab58e669d95d8465a625db90ebe26",
	"0x1a57EAE941e55DC219Cd456352c9d90035d4d41A": "77358e785fadbce9c1880ffc72aac79edcb93edda40352270c114f63483ddcfe",
	"0xa1fc5571BA8344E96C72bCdC792e0E9fe205E6A2": "f45a8d634cb6dda383cefcddc00e7c332a2359e47990bc263bde27b64530215b",
	"0x8d9A9cCd185b10C40e27A84bd4Fe8ccDE26d35FF": "3a7da279cd018d6122bb7788d11e83a4c815f108de4d2b82dd88db64ec03c128",
	"0x37E1Fe38858ECDb2685DB2F17499E7Dd4f189529": "b187f744a7fd0969751e8ec344e259dd235f8d05181388d29bbb23e740da5a37",
	"0x00B1c424736af61384447A681001BDEd424Ff7a2": "a1101844292260b5d9d69c8437112ddd9b8a899c07941b81dfff095addcd5e85",
	"0x39450Ba610522aAaE1d002AA033Fd432F33757d0": "8cd0b1fa59564572758935347bca99d0be0cdb1c53fe7e17f08b3292e99489e1",
	"0xeC7DA1c4268d04200A4D0Cc9aa7e80c719a8FB4d": "b5667c13c59701f4f754fe2a9ea0d459182e887ce93047334f2e804a192bfbc3",
	"0xE709E4425B1047bC732Cbf6D4C759EFeE784A99C": "a5dbec7a86b39b27cd30d3a7e3e3a75dac38037eba8f0bd4a394af8d929c1cc8",
	"0xBeB6Bd33AC1b4146aaF946021F15363949031D53": "626f2f3a20f800414f61cac7282ff85bffcc6b2780fbf04fe0f4fcd871a289bc",
	"0x7681229737d98811c22bEc685dCA77a5E3935D48": "83bce2bd54edfc773bcd6cf32131168b7364d163b98414f58a4981621a347581",
	"0xF7bdD3358832cFBF2D7D16dCCfC35BF1d64f786F": "60c067a4128bdc7d5b67648e78fe38ab4b6b8f45749f21f20ea8be5dcb0715b2",
	"0x17E6e7E1Af1Da51Bd443D23C4624816356097FC7": "6c330b27c890352016995e7c8c58b64594d5403372d3d93c0b7107f13455ee56",
	"0xeE701975bFD65C7d0A89412DC38F8be1baEFdb85": "d78d2fcff07f0caa4b6b46375ce054f960d60fcbb05884d72fba71fda931c3c5",
	"0xfDEb7AfBCa2771DE3eddbFCFc7e70Ecc33A18957": "b89541e727950b0d4b671078741c1f570c0eba17bc66dfcbf2fc48b161f807cd",
	"0x09378a640AfB04126fb1817a7CfC03D265b3B8Cb": "af6bd0fc38b1a8ee5927e1756c292691b5de65abdaa88ba83436d8870ad01f60",
	"0xB195C652313F2D9df50b03BB239beFe63c440324": "3f16c72ee7e7e8c747179f517e59c6ce7491aba7b9858599138e494b7f4dc835",
	"0x0031E3C30Eda319bf6790a83Dff14006d859ce48": "2c2b653c4899eb8e61679ee30a9942e3052c986592eda3da79f4857a1a14a75f",
	"0x9fBaf00672913CC057E371520d902A7ee0F2e25b": "3bbaa55a22ad38246c69def7ec2d5c15b7bb5fa34485a0a641a3e26279a08655",
	"0x192b39199fe8445DF8D1e263B647A1A0489D0f73": "2c148a01249630d63cbd4d9a69e99c8bfee348880a3b39390971a97b14592ac6",
	"0x9701703f81b4188EfDFc277685400439aBf3F793": "ac02f8e9927060457fdf4a857514fb4d964fc5932b1af47a7fc98d9f1da057ce",
	"0x91405A9D6067420c6448755bADB31e28977d940d": "ffafb99c5b01439127f964ea4051c2ff7c3e762b651e867c884523a218cecb21",
	"0x3e19C4A0D5c72a70351FbcF363E1127A4e4D9489": "ed5a8628e6c4af8209039820f90dc21f3aafe0b4d2ba224e0fe12b64f2ebb9c6",
	"0x88155c923F30Bee02ecE40a7112f43a63fd79Ac0": "df349db80f1111f445e928355d7211f84a3c3a230815125ea5045bd8c77c2e3c",
	"0xbF296a6135252Ca2626710b7662753b91E86f136": "89792cccdc24daf4af3ab9bf174c56642917ffbb535ade087ed76d5232c3aae2",
	"0x176eb379C47704C6ca109316F3e20De2F8379f17": "8aa2314050ff6377272c2c73e2e812904bff54e4111bc3ddd4f067cdd9b74e75",
	"0x12c9ccb7Cc1a502027069053674ebaFe461B74E5": "2264538ee135ad79961eac2dcecba1ba078b0b51dd1115f3da261e0291f99d2b",
	"0xAb5Fcc4fc96aCBf846750CA89aD37D9949c14199": "3c9d74ebd4e5fd37362d33c96e6e864efb13a14243a8c651599dfd1a2a371e84",
	"0xE72Be2b990d9284CAf7d97A5dE5Aca6088D409f8": "b56cca6a2b23a770286b37fa6c61fc7a7f24f185362ed5abf043554b7bd90ed1",
	"0x29f2c85318E9AAFB97B5A1077d03abAC249EFdC7": "7ed01dc3ca8e3aa86910637e52d69331170ca688e010b69eb5858e75ec04ba54",
	"0xf5d8A073F3B12fBCdeE83dAe0704d4B318f42dBD": "33535f615e9a59a48b1b90755e53a99c7b10b70a2509439f9e05ac714fd14040",
	"0xF45251DEF21171BDC495ECB01214C8B54b8bbC5A": "4e11e91a937d10432b2039e61b3bf3b14c27317548d57229e78e15a777f1a83c",
	"0x6E26Fb2A1F7F2741DDC3a7588a3827b0602247D7": "ca2fc3f76e461781b1934d636df5b688c2804cd76d727dd99e2fafadd541e42a",
}

var GenesisAddrKeys5 = map[string]string{
	"0x4f36A53DC32272b97Ae5FF511387E2741D727bdb": "63e16b5334e76d63ee94f35bd2a81c721ebbbb27e81620be6fc1c448c767eed9",
	"0x3013d14272220C4654ad2623CA4bB6D65Eed3858": "bebb0f2541cbc458eaa75be97d344217e1ccf59f1fa42eaa5e59d2a5cbc9f6b4",
	"0x4ACfAd4c4E92395f81252DcB5cC08e09ED21BD26": "e4bba6368a7294d303cea4a7b2de2b1d9c43d55b4ab9b5d0edd4228f27504229",
	"0xf07A33Bd5982d974ad14AB01b63148359Da1518D": "aab56ace85d9ffb24cf7439cacca2a53a9e8e936cabf844f95a66b0f25677e1a",
	"0x347bf6D78F87c3cC268C1FE794D841C3AE5Eb3A8": "af1780001082e0eb964d89742c5c3e1002dea36ea3daa6ee70d6685f37244743",
	"0x659f985c71260b0917B376D33c91984Ac0C14B3f": "79559d47a673c3e55784f6f24cb1c1bf89123f6350389c840f3ad88b6317bb00",
	"0xD8815f1FFec9ed357cb23E6e49D8b4CBEE982918": "66434d128d5936ada6a7b5edd20373c220ec5c6dcdddef57df4783da5ea4f792",
	"0x154b9a9D6C16b236901548f218a6cF6223a8D297": "856178bd724174dc8c67cbefb0c22a2b16300aebbfeb37d0ab48f6705d8acd36",
	"0xd16FfD214942b78F7F7ac601eD4D1C30C84490Ec": "4d91b2d7d45e589b1325fa5723a42681739626f158c4aaa1ada15e35925bddf1",
	"0xB897e32114258a6b821F1F7ACCF5931240222A11": "3bcc1f9b32d7643e9e5b99d63c85dec30785d553c3d23953eb00928ccf655876",
	"0x8535C9D584f00185b2Dd1800d4852847bB85D6bF": "8f529987f7d1a6f3efdf06929a9aa55b5b2baea23d9d9308d36037dc64965ba0",
	"0xdEd895A776a189F978C85B24EB3fF8d5b310fEd8": "47c080a370915d68fc61ed18ca0de80397457438fcc199b7c91a5b42a0592e60",
	"0xF2dF88c8398cDCc1cc6610a51a7b713be19428Be": "20d11ef4f986f3e5e343ecb9610273f2fd0e371a1e60e79be61521eeba7d84fb",
	"0x35dDc8986ca45BDB8C755982cFaFF031634EcCB8": "d709de3c36331d02f35e39aa9495ad69ac1e2416561e572d4ba8d4fba09ee3a6",
	"0x5B4cd1209b51A366364E514E68b1f2Fc581937Bd": "12a119061f0a49392fddc2dbc0c41e864369b662a218dfc660d2e9fec1be8a8d",
	"0xf6dB4C5fB3bC858714338d55e009698a667f2aF0": "c8b1444f8f7c1a667dfd8d15fc46382f7969f51bd462702d5bad661433dda717",
	"0x469f8AF52EcB4a9d4E358232F7ECd10a199777d1": "e0bfdee3d16159d1d6e5768cd08905cf5b4e40010ac7cb2ade9d61302c9738fb",
	"0x8F151FC5b854C976Bb70B154CCc13FD14dF1d3a5": "ae3aefb7043fecd47d4c27d245088c9efd4e0a72861f6ad778bdfee1c0580c44",
	"0x9765CBD085E692C00b81abC1a064E4A15777960b": "470933eaf78d3c49931a1126239725dc7c3bd143b3f87e76d1c2fb9cb1ea51fc",
	"0xb107A8E037d2D0fa01169c1fF991e6E6D2C17Be0": "c10621e70c13ac2675ed27136765f04d3492f471a7f5375972903f5d4e5e9ad5",
	"0x1b20541F248D53CA403445c630C6D619C811926b": "cd56ca0c6628a8b408bb65fc7bed46e40003a8708e301c2190eecbafa384766f",
	"0x2c0603848c16B88A36420EdE7aDb78c9523380fD": "35fa1a953351f9a1f80167923f28c075aadc8ff42ab0b4b0623bfc4a52e0d6dd",
	"0x9Cd3487a19680EF6F01F9e02c10c6339A9412ABa": "f7f052e66dba71476d3a095ab3ae09251900ca905c3833ffaa1ed82f67b42746",
	"0xF751b5986764918aB24ceEF919681A159CF1C492": "4a9c6283c917994740946f01bb6fd62193e508eb9bbbe266049fcb5549a165b0",
	"0x98db63a89Fe8c54d7f8DDCc00D1A6Ff5554137e0": "2a96b42c388dd3fdd558d62c4efe32855897f06010ba4d8ead452b82edd31e87",
	"0x30EEE9F6E9Be9a55eDFC7765aB80b866bfab3387": "addc3d041c0c6383bfd3433c73d56f54c4f2ed39a6ceab534330283548d37c32",
	"0x2b9e2CF96E7F7676df1Df451C6Fd807C5f12e729": "b74fe4f73ef4edcd1c4f8966f9e002557036176a9a9e995278709422ec281a18",
	"0x82A59Cb0182E0C0BA89eD2b1961E67b50805E742": "dd03f4be6880de2d2c98a6a901c2ae8a2ef863c95c58835cdaca8178c6b39efb",
	"0xDEA515C9CEcF292C39a8f4c7f185191674E5e23d": "4182b4e30726a564e4db617c704469bece57a41cfa94d1b309b6163dfaeab940",
	"0xea31feB6EDe2a1F9E15a6196BFa1B3b01700896D": "fd3e1c09d8ff6cbf4a91aab02609062aa8847d6c26b30d2904b07db0ddac0b26",
	"0xF93D95fbBCEcC3Aaa0E2e1AB33CA78df67b9D663": "8949c29cbe438eff7c7f51c7cbca88f57369f3f73ba63d13788a12dda742de8e",
	"0xeb68cb2ED3D70463922d4b78699B4c73142F73A4": "74a5b9b1baf48c838a269d0eea2e039b7504d9d4fcb054f3169944bfce58cf6c",
	"0x03628904AB1b784Fb3cd00359ef40faBED8C055F": "5215527a2a91d81bb32129771cd8f77f0e6e3d44820fad07f1dd849c586d2fd7",
	"0x3BF1aFa1E387A9060C3fA9Ef2920B758F2Eb8AcB": "79eb3dc689449431e42f696af78d5ee1de6ea31cac89ea0705f1b02745d24fc2",
	"0x393Fe03a0868c268DCaEcEF596b1D86745D3Ef47": "9f0043c99f73e624a55061295ddb95b83a867c85038de4a82ff8ae78d087025c",
	"0x6D16cf8290D4826c2e23970cCa43F2FF19F0D36e": "b4e96dd6c2eeb915f83ebd135a8a3375776c1c96507d975c119dfa244b1e0d61",
	"0x430A827EAe164adb1984a44F953c13772699AAf5": "984b97645c749c315a1e3ba25ab4443f2a521adca4914bf3279cf7f80fa4f424",
	"0x084B1A57978Ff4D8D85a426bA695aC3E1C4Bc876": "a2bd15b69fdeec134496332dc8d115aa98f87f98e87abec32ee554f0100963e5",
	"0xd87265C8Ce2D2463293f571218dEA3D685feb396": "a47b8f0920ad766b033aa12108b9a11b98de996fe582434207c3a6e6e76fcb2b",
	"0xCE4d69809506777477937C288f6Edd5f7F963483": "ca9f358264e3f0b70f1f39a2c6a04bd3d9e8640a3f09fa34f65e9f3d6b97c40a",
	"0xaBB1A6ea4b23D1a4FB3072DddA5e7Ad6088871CC": "c848faca8e51f5b4ddfa95b064fa394329be6d0d9ec8ea3d0ab9246701adb637",
	"0xa88fb954C857CBB49605e9B69226a1BB77968F8D": "fe1a1945d9cdb993124be509ca9687cb543886dbcb6a12b5ae30750c689b19a2",
	"0x34128D82Becb2BC19bd00ef8202Af2cD3343Ca18": "861af6770fb419095d98c2d7adbf467f0d6f41d833ea0fd7370dec0076a3dafa",
	"0x273a4Da86FEDC61A762FCd459e37D035aCeb2E9A": "58eedb81e2d697b2425b40c03aab4c25495faf12f2b2bd80f4d37720970bf857",
	"0x85daaa16D41cC06b3AeAB2633ED5945fdE1170f2": "522fe6b0396f8fc5f57870f538fa8f9b2e2059f76e9599efd4a1d941302c6dad",
	"0xbd04Fb13b26E4cc102cC3e87A97809e6D970bcdc": "5d8d9ecfc9b771c5d5df2eadaee727f772f31e90d68fe6dc897e683af66a3d4f",
	"0xE499C3Af337860EbA7c326d55eC5515aD03Df236": "c80cf7196bbb8541cab6d9f6e5d20931e989c1fe8ae252a06c8a7dc6f573af1f",
	"0x3F6f726E742ceE1d4762b48ef8448DBdFdAa3996": "2885a6db5d2a1bc0e9adbfa23b08a341c7cf2b9206cd52f60b5fdc82d5a0815e",
	"0x517c3622D06f1942096a633D941d4CbEDDf4D881": "aff5997d71bb4221b80c18fa548b5139ab2903638786d9111a97a608b1aa344e",
	"0xD6391141744939684422361750DB1Edf37276f81": "a85b718c455345f8b019aa81615439244d80ac2a4074fff6930e1409b46ee5bc",
}

var GenesisAddrKeys6 = map[string]string{
	"0x3670d4E841A0AdC6E08Aa6F40E430D5fbfE390Fe": "c05d4c746505e59ea1eefea961bd3856fab1effada30f72fba074f1c47bd712f",
	"0x1347Fa7F4a7048Df396233a289a6AF0403C1F952": "aa5a9fc053ca63bbd1e9afcbdb39ed04e917b1b3baaaec0baa63820038a9d824",
	"0xC0a872C458A1DE6ddC7027aBE2522C280edf6e6F": "13b7d9053a5b79e2714ac94505e13ec3a1d028ab0925b0e9129cb1be7fc5bf69",
	"0xfE33Cee33041654C2BFed4FBE9bB1C43eb0eb3FD": "1218dd41cf3a866590997988898e481cd914a4e8c1f88b044d7a8d85711ef51a",
	"0xe167bC277257030C7B5B032888F13cbb06b924e0": "6e68422585c7d8d9a342e3e30c518e2f4aedca1485b75bd473427cac9b3225c0",
	"0x8d62D733393C20A789CED16a45F047115A8F6699": "6225631d1221c9e6a3cdb755bc5195a4af0d9c79254da80a74942ef47906c4f1",
	"0x671FFf5fBCc577e610b25484553511206fD13f8b": "4dfb0b0f8626109aa5edf01a0762bf7d565da3fa0c67901847ad3885ed3bb1f9",
	"0x4Bd0AafD1780C3Ec6903b17fd3B5A28507A21568": "3c870b4967e0336a6dc6e66bc69b84622ab09f0bf5425bf3847baff91bfc7e7b",
	"0x817EE72bb1B1b730D9A50CB1388b2E8eB3dA27b1": "5f8244c3d6798df2bdc08b3ff3122d41461df949a0d1aaf6155c43d91d729c43",
	"0x0a2bb5665E3cD584852C1484aa4085a91880a4C1": "742e21ae94cbe55ce8198d5d741eec0dd438806257be2483fbb722f82231c6b0",
	"0x658C6436Dd43D029cD2d7625029B887A19f27e71": "b3aff590b0094829291e4de60f8cd1c1cf74c7d5e215fc3faf32c5ff4995bf05",
	"0x57c81659247C8192C959cf211FE4576bDdDCB207": "4862ac757c02141fb60777576a1d577c3ff8c2f6d68a4f5ef91fff162a9db0d7",
	"0xd65d66379a6B588e11935b6bc3895d1485d6e982": "193406d0b50d40fd9fc0945374b15d883a5e221c2d9b27af70a428209f96bf2d",
	"0x456175Ae8e2139E13Bd1F90f42A7715cBeFEa3F7": "ab790ef97f924b3ee49384b74a0702ed3bd1a2a5e7fae84ca0c389d4a9515508",
	"0x61B03e0d450Be094f81071B7a696424F4A4f3Da5": "fcaaae4bfbaf1035a81db8f7cdbd47377a434aa6d4345c5b95d027fa54e4ca6a",
	"0xeB904b9CF3C4B3d336e8415b79E099c1e38F0938": "d4e6129ac2ffad40f916a2e3b69f6f1fc16883263cb1531436d8343ec206c8d8",
	"0x4468b6d0288695c04Ba8eDC25971c355711B4b0f": "6b2b4040187f3a02d6e46c87b3a24b5fa37b0a6993de5b98cd9704f072095d9d",
	"0x8391E9DA8f76424aF748550af419dFD3EB48C547": "77280359591fe9fd364c253d537a80177423ca36eb9403cf623fc8c39b95c55b",
	"0x5EB0ee09220C6d20Eb339C1A0332A0aF65E88c8f": "13232b3ec908d0f309a01dbba84bed9b8e2d90985d2bc0688e15d28b55f27020",
	"0x66dB9a93E95D13f3fc129047643d572b3E9F5BC5": "c3d1416178ecb45f1d77245c0ae74b2ff9f42359ff828461fecea8898c78cf7b",
	"0xA3D996d84d158F8455b2f8F75311464715504Ac9": "5fea7e3bc62a74f03dd21c97f941ba4015b55aa6fb081e278dc5390291fc6ab7",
	"0x8D82416d8E93869fDA2E3c4A6f662d4Ca110D85b": "daf9a99a39fa9b6def19d58d47be3da2be46331548ba918981981cdb550e0877",
	"0x0beA13364Bb2177dc4b0Be1421F48e362D26Ce40": "c0f6e7e5889ecf7fba2ac96f965a693bb2d287647268a3f942ad8a2b90ddf388",
	"0xc647B36073B7b77C539386353C4f4a20bB5c9Bc4": "c351764968741d415ec0946b6da0e34a2fafde61e8b704d59b240759b15f42fb",
	"0xEacEb177F268a8f57f441e596a800F6aE1F0caA4": "b294e726411749c101d3d9f900e06fd130982f5da65f60f9326b94a0d26fed51",
	"0x8272ef99c6c28B1732df6980db620C829D7C5650": "a4fe414bcc0d0a38d57d6b9d30ca26f49d65cb4ace72adb7016031ea06eed3ed",
	"0x32453F63480196Df9875830b952c7cE0d2C6f040": "3f459985f3a3614f2d2a016761c4cfe0e2d6fa35bd0af0567d173c24d4f34dc4",
	"0x8860E7450ab5795C60B09b05c9c0aef909ffC16d": "bad7ed6d12175a0cc96f7c7bcb15e96339aa4705c1cca7232e82e3e3eae02463",
	"0xe6d05064F30c6D28905c5fF9393f380d7fC6e72C": "4eb76e1b86adc56503c4b08fcefce5495781aa054611f8dad2b34eb9af8e1c89",
	"0x735024Ec4ac939B4baB6983E9CcEeC90700F8cA7": "6a1d6f87b63888616daef4c78fe118fe4d33e2802ffea5f21daa2995b7194064",
	"0x9FF37978CD4EC0720ECD8D0C8eF79296407cbD21": "bdf922722630a94a2804b0a8d3a845e704edee394af30293402cc4290a99f84f",
	"0x7fFA5A9f7d0FFd09762d6849BCFC5b8FDEFD630A": "cb685405069f46aeca65626c62058a6595f1b28386f0493832356543ba5032a2",
	"0x5A8a70884c83548122f0D2c8718A9aF8BC657477": "2573e36209c09a57a964732996c911021c5dd24cd188d370ee9c64ff5a72a373",
	"0xA730B4DeEDb36A896F7D77Ff18195941FDa1E066": "c0b8de19ebfa023229c933380f101b6385c6a4abf62372ca58c3220c2650b465",
	"0x92CcBdd5B0a2fE4950DD99B448D32EC7EbEad8F3": "5a17595f98b68e34fdf6df0a88dac2399ef0a0c9f73a98c5960bea2358d818ee",
	"0x343DEBF01fd6143B48C3DFe5330A26169Fdc67E3": "8ca39f4a7814b5586ec002fa67b3d5cebc189d340670ff6e0bea18c1b081d7fb",
	"0xeED66e966448417e96a3c58A74F1851423b8f626": "6ad268bc6a8cd14ad70fb4c7eb39ad9eaf9c6fa30ae61395c218e8d6365faca2",
	"0x527C3d6bBaf5551d8767b81EAf7BFF5a50A209DB": "781bedde711666dc841822f23cf241b8c936d2df030aae750ef583c5b679ca25",
	"0x885D40651Ce820a8459E4C4a9Bf0c7630413E0F2": "fd36c7d07f2e19e3f1900eb7e8a4032d6e545aa89a51606b41a135109f52bbb4",
	"0x1996ec73BDb9eB2348A95F3a4664C336bF651bA9": "162bcdb4b1b7e31500dcb15d54015d8daa96c5296f0f2a8531c097c62b0dd6fb",
	"0xDff7E2F72E67158eA64Ce836F194Fb64593F6630": "d169b496a1e8101a4f906ad5bde3e1cad1a4dae9e585c2e13ffcf7ff739b5a52",
	"0x5fe3672235335fD93689220b4ea7CD7Ad34dE3fC": "f6f2fca091a16cd89d17457411e3d959aec53367b52250200824a29895803f05",
	"0xBf0C90C7eCBA6C517e85d9ebc74F48482ab3ace6": "630ceb6641ef3e1f719b278cf82cfd756132e7ab055093e656dc7505bd70bed5",
	"0x1f2c8C3193C2b1b7C2f92b83c4c346806586e105": "e90922d3d01a3dda9575f98622d1d380d604b8b7cd5e1881169092ca19b4bae5",
	"0xf314322CBFba5794cC88812f07d79E0ACe5bB762": "a6ac12309b62b0d7c53d6bf0293bc4533f1feee6ac6ef984a87376151e8c2e89",
	"0xe94eE2bdDF0b9a23412F8e3BCa08b508a711017b": "5fc8fd27bdb8562eda560aefde6d582adfec1b43a07f219d4c5b6cd233eee33c",
	"0x18274E21310DEa944f3079d2aeA12101655848d5": "412b1b97de6ad0d757d61d43dfc3ffad13a3e84624588e1fda10b63ee2f03a52",
	"0xEB5C6758a3c19245f2c99cac4471e613185a2ac2": "d1a760c642cc1487560bfb344b26d4e915490d1190cf5ac70d2c20ce3327e20a",
	"0x95FFd8F3bAEb30215d7bCaFe5DEf423AB9454ad9": "883d454ba6f4aead93097887385a851100316e1215e8392208af879e710037e6",
	"0x6372a0E02078d57bf81e01FC5deC45eaD366AADe": "aefe9b3fdf99c0cc2ab5dcd2cda125b3f7c939211986295be307d77abf24286f",
}

var GenesisAddrKeys7 = map[string]string{
	"0xA07A5774F8Ebd2C786efb367dfddfee5e6CDC53A": "8719889e30776b9408633220d511c2ddce5bfcd69f1fda10b07bd4eeeea6ba75",
	"0xa35d8c3d07e2097b422bCD7589146123B66835C6": "3666d26eec16ef3c1d52b9c676a55cd78de9bbadeba31f931397fa1331100875",
	"0x331Cd21d7Ef4e1FcA028e492a2Dac66706d30a50": "808859ba9be7fdecfd19bfdee8af33b5fdfb5e45661e845a0e4d5c06d6fb5c02",
	"0x2B598b6731245E9d23CA780205A26604e3C999a2": "55abe760ce34e7df737b83c2c72d2a5ea37db9ac54c043254c1c78d8b4d60941",
	"0xd8E682438a84B391491734C75Bdc104BEA2a0bB5": "5795af90957cbd0dc12278bcade00ea89060bcad3dfe59d0a23081d8336d4b9e",
	"0xf49bEeEdCdCa43f35AbD1059f12454eb7c43e007": "2c628ee8180763eea0f128925fdc4004f8a70326aabadc1173ba998f62b6b9ea",
	"0x53617B38405174518DbdAbC3A650b39735daC735": "626a54cda3bb601b04528a5317f2f66d22fffa56f791e1d9acd9fd920cf3cc3f",
	"0x9F37720e7Ea8103Ef31EE9eB14c5bef51a619512": "c0347f413f288ce3de1379d7f7e305d0872fab0e728a8a056e87eea836ca56a0",
	"0x27d8d6aaF8F099B549b4047DF9ba35DD002fB59e": "5308350ff9193e444e962a9c91b6f00af9c04e57dfdeb541e3da67dc79601894",
	"0xfEFd8a7EBB93335594a6e59a63f283358e92D1ca": "470fa5d5742eefa07d9f2c9cabd236f8acf58854f8df8d2bba3df50c47d3e0e9",
	"0x21039d05771BF4f8BAa8C497540ACbD35D6Cc9d2": "a32b88592d07835066125df6afc2914d277c506f5f5d3b786fc3eb6905ce668a",
	"0xa85Df4Af0c3d4CA56222Da042F9c07940d5b2727": "a4fd1a67035c0d1f5e747d066cb3a7d26c532bbfb3119d0496d46cc3ae38a3fa",
	"0x5816fd102cE4c87C9d6E36b4B97DA1AD9e09593D": "c1dc9470fa05973938c860475df87a357d17859eb6f18138f1be8300d52ce761",
	"0x1CF86F387A3DaE2868AB2cEB5306c269A58c0b3d": "188ffcdfde1a78ea60b2ac8875ad1120f2a6915af64c7b058a20609a175edd8e",
	"0xE3BD9570Bb1287138FB02FF4aeC9C057e1BD7551": "3d565014b39300f1a6a23f902aa329890c022dc4c2060ed53bc3cf2aecdaa1a4",
	"0xE1234156D3e5A420d2A552723a31a915c2c5966e": "9cd5d3fdbb9fe47260f85d7ea1f4056a80f5573b9f916db01b24ae000d40da16",
	"0x16B8c53112F76947Bb057A4563f565ABC0a301da": "71d6cf2e5a6c22f93477ef82be5cd7d5f765077c6744a94708a2f3731a2d8d62",
	"0xcac727aa38DBdE3467C8E946cb183b7Ad8cc04Ee": "23717a4cf16aeb6f1e2bb9fd0a6c9cee489e9e930fcdb33c7f3698bb70ebe482",
	"0x5079f1E28336F4C3B1a91FC8C8553C0f8aEeFfC7": "466164cc7bcb808c03faf0ce18a04db7518307fce6439f9198591664c4827a56",
	"0xaD3203923e5993A20c30c976D67f79255677eC96": "40dd56bf15adf48b4da25fcf4253fa9971dea2112a8320af74c558ddfc3cf2c0",
	"0xC23E56d4C3eB526756a0d7DAC4C45085134e79cC": "4e9b483d36a6cade3057dd3d9c39b0a2c6cd186d84868b9f8c294aa74eda8a5e",
	"0x4B6A760Af1CBeC59cf9B513225BF2d1ddA4e18Fd": "1a522f874c10ad1fa3cf67acd5bc86898afeb07e6d817025b055bf2154a02504",
	"0x9659778Fa650411DA6e8987c316c26a484190518": "b5183d6b8be482b9fec2d8aadf79ff488830feee3524f809a98ee62b0e14b4af",
	"0x5BC44895b7480181d9BB9E9d8671038477BfDfb3": "374563faef3d0cd93d9067c131f297d0a0ef7a76f8342c8e27ea5e61a6e8c7ea",
	"0x772e2E63cd80f50F95cff38986cB10170CC96e41": "ac1c3b34d6446437c4237507470f0c8e667ca998caa45ef6b6210e7b3a784229",
	"0x4A6f6085ab4AAbcaAaDCDcb5d5D6816209F99F3d": "3b54526c1b958078bdda54c473bfbd5ecd2dd1cebc7da55d65ea03b517001cbe",
	"0x221EE91E5DB13bB170BB654EB357D8D1b47e62c2": "eb43f95a97ee45c6693ce2bee7672d7ddd2a13f0ad229c7d6ec5011c8996e915",
	"0xB34a8AdC476E671Bd45A8375F54BC3Ac6729D146": "3038f834fcad742429f826003d0dc84038fb939735b4c7c1a315c7ac394600c5",
	"0x177A299C364bbA3148E0180770601427B34C4279": "795397d8f3de229287b496b37a390ad7a3722a6800074fea60364d5f439fe864",
	"0x92660Bd3721AF3812E3f4692bF477054a2903a7E": "77071861439d08379aecca078ea769122eeebe5cd15b310317939a99e8a80bf6",
	"0x1Bff6Fb01a1057B9D53E57da7437dAE755d2B966": "116cd867b63f1872fd0d3bb5ed61e2254a2eda0adc1a679e479e702eadff03c7",
	"0xc9e3249E8D47167E4287a175C20C0e9F9d80b934": "cf7ee1ae9ea2f200250c6e728ca5e145323b3920a2fc42414cf125fb5c48ded8",
	"0xD824bebf8b8565542a7495Be1D4F5A38a731a91F": "bae1ec01a1abf5c3347fdbabbca4f1116270969cf1d168ed4bd156d383756217",
	"0x0b7767B4a96de1d95605d27594519dE32b054018": "4f43f37d68a37a3099ad740fea06f6c882481aab46fc010f8109f52495b5a214",
	"0xA65c160F0765D9aaCED686972ed7c98Bb135491a": "3e9679623277e60e85867eb2ce98df002d6c995ee9348f6a11e9bb557f6d4eef",
	"0xB27aae37278C934A77d7fbd39f3aD11d0C1C2ccA": "c294cd3097126b083bcb9bec141b14eef8a483c0c9e90ae96ea13f86f8e5fdb1",
	"0xEf77C2F1F797Dfb42cF2D6388Ea6197386d98CF8": "e4812d242a9926e49f9fc9539ef15676987d100749b301b00c06cae4d9e46f32",
	"0x35353CB9f05CED2569777bEa15d29CCF8A19EC47": "989cb92893dbacb4c6af51fbf31d5564b3fc184d4c2d1c11057100a625c73a39",
	"0xA4762Ef75BB581E9cB51BC849Ec365542c719438": "948a1c7f17f9173060a2299b2668a300a31310619d819f3207cd73b68a4c13f8",
	"0x5A8EF098F9A6c883dd6421250bf9Dc91B587385B": "d20c617e2f823898bbe3d9c676382454bb322bb83700fa7172115db75ea4ca0a",
	"0x7F7911556f0Fa0472e9582cb2B7877773F9C3B19": "ff053dd2f5858e40d9cb381b9b9ac1cdce482a6b9db69db51e3026516df663b0",
	"0xb88C1800AedC31966a292dc2d921ec3Db2522862": "17c356c57688b2b4eac43fe89d922add8e3d013ce7f033714a3cfb2eccc5b2bf",
	"0xe5D8480e529D132c6d55bd442A304b6244c0dbB6": "f03a4c70050d7b2e98a16634ca60a0524b4eafb2478b0fc5530f9396b6ce3da8",
	"0x88e6B9EB555d1C34C8D4d090CCF1071dBd8688F8": "2ea78f1e81a9dbdd9bd1e9f96a911c51f1578eb7c575cd913843949cc1f6445c",
	"0x18Ba59A8B9154806b4bD55daeFEbF09998b8FfE4": "4aa0c1dc97cb77ebda8584853312e48863eaa0de4e666c49e98165ddf067a26c",
	"0x5897029b2EEaAE2D971848Dfb9851459B0d74193": "4b7c3c2b8cac63cae6a2ad7070dc2a8e1770abfad469a0b1581d97965ffd5f9c",
	"0x27243418429e0847D4575e8E9f7B61F5c9A09fa7": "7aab4b9b33caab9a6411612a090a342eb1b4f6d5ff696a0e77f0f380f738e5b0",
	"0x4e781cc6e04cE76225991C0C7b7fd0892fc62955": "9395a976c1fe16922069272d5b8c28ea4497089b0921a6fe826ccc572a578729",
	"0x7a99269a00A8E4dDCC43bedF19389565A2fF0d91": "38e0c8e1cdfbfd3917b03d0bbe38ffeec478ae699f452d1d7a2e90fbc0934214",
	"0xaC89b0d5D6b3Ea76072aAe079A037367fd01DD1c": "2d94cef05c9368076f3b1f96bd0c2f252801c7996b6f8e5224d2da6a46eabad1",
}

var GenesisAddrKeys8 = map[string]string{
	"0x5AB759881d027561Fce51B422deE6A48713f4d80": "c46c327339672b1385829ed0405cdbbd9285c58a3fb0657e077552f63fc48a65",
	"0xb23155D0CF067874D3206AE47429869F40BAcD81": "6de588a7a463061aa8519a674b206d3d1cff75a21487516bb3a4db67421bfd41",
	"0xb4Ee24dEd407c36D7dE988Df1556B43B9ffC167c": "24f8e53cba8bc678d1b02d2a5075f0cc7c067709c0d183231c8d8d8a1c671784",
	"0xfEb5b171249905aDD90eA9ab10865C72601D54aD": "6b6e566ed82c8a5d102dfe1732b9318964c6cd912478b182eb6c309e1ca9eb6d",
	"0xB050F1609E34E40321B64b18a202EB606DbA41a0": "b51edde8509533e76d9e785c825f901fb19c3403ffd0186e34f86ed0c74bd360",
	"0xB5dF1b692c37799f775f247c0387826710a2aEcE": "9871f0ca077bff2e3f2558e9bf5068cfc1611aeaad0b60c5b9188544aa8b1915",
	"0x0d8f13df4E7bD39f206E6b49b252b6e43C0FdE87": "c00cf2fd649674e2cf69c252fa76e80b1718f08632c748990fbe178ce876c5fd",
	"0x9890E838967F3946F54524AAEaFAa862D69FE515": "d0d56b5b18a4f9173808a743a825ef30a6146afc67bfe38d4c58a1d356fb3b97",
	"0xDBEc981FD4220c380B24a8E7FD3Ee47F79949325": "4986126c7175e76de9daabb6a5feadf2fd4e1fff39022694304f4979e4567af6",
	"0xe47cF6bb6f943147c03f9200cddE61c7Df707dBe": "3961709ca9f99f1d5ac9c670f438aa109d67e908bc3d762eb474f50484b57708",
	"0x351Ad913B02f0Fe95757d47c1d206F1bbC57C0Cb": "cda57d7ab22bbc111fc3907b61ea028ba143057266f5e94583f020b69481d261",
	"0x8d044A5E890B9F842376B55bEA12e4A6eB67db59": "2886aa6c393cb035608996828607c321dc852467cff5b3d4f0da1c3931db3a07",
	"0xAEDA02e080335F1B6Ce1313D1b5Ba1fC79ee977e": "619f3708893e41a391d04d4698408d1171265a67f2f2a80f8163aab43a788f92",
	"0xdca6A02D1B0C93578bd0aFC4eEd1D97B1221F862": "3be313a4a1f551abb8ec2df015f4a510ca9154cb400ad6a238c703deba2bab4b",
	"0x840c2fe44b88af27CeBE2598C8f8371c3735b59E": "13d28ef766f21f4a7a6103472468556679cc558ee9293eb33792e3f833b073d7",
	"0xaf135d99d0624206A9957096c8e00C7ac9501F07": "5baafabe1517a70a41dd6bee1dd4a7b6c0b4c0ab5374aaf20041d0cc7c1918cf",
	"0xec9CF62dDC5EC8b145aE10Bb0e07874e490Be123": "d5c881b6ca4f2edba6b33fbfd6081435844850b6ea77e919854bd83cd29ff0d3",
	"0xb90735bE9dFEcC8101f17B6D2E8509B0a49A0496": "164dda6daed8163b6101753ff6ed8218cb21a9412743a90d9b74eeaf1307405e",
	"0x25CE013F42839Bb09BdBEBBAfD88d33da4a6FbE7": "e1a54e0ea2dc68254404eacfa59fd194b66f28481ae2775f4bde50034e885dfb",
	"0xd57BeDE413f41E73121EAdF9F150EaF3776F7fF2": "2e621711519183dccb37004c10572659c2b10b8fa7c9a90558964ffdad55cae3",
	"0x9958f8aE195C35fFCD35E7321Db602fc2B35995d": "c5496bca92bc0e3c4b1d433a069271a2442c064b282a5b8a9e4cb268db2b78f9",
	"0xcb17544CB20393E8e85A0F6f667c4c82e6346c6e": "21683077ccb2a0f7e6873507c4e396067d73837c7ceb260c1c32c096d007471d",
	"0x78bf32Ce0b6C0a8D634E933182D6A40A4b584943": "c9ded53e8b85678f1ce9796f3ced117627db2500dbca307507157d5a46bb4269",
	"0xB95f9E6Ae1b69b8c96CB53bD5487E8aF17991eB1": "bee14693c59eff6abeb94f24bcc10bd013e4435f72ec30aea57bd866f74819bc",
	"0xEabFD008Ed71E3FF7c861dB4cEbD54F5C2B6f381": "a91aa1782c0a675edba81ca697b4ddbfdbc89461570d9e49c05811544d195b9a",
	"0xeD007FcfD6398C9Ea4B04a9F5FDc5F3CF5Bf2394": "bfed0fa8af2d4db4f968d44bad276a3c9340460e6cd75b1033f6ecb0c635bddb",
	"0xeF1F611d871Dc539E7831CC494ed4d556E15634B": "b1be67de2506ae4582c4c1717b7a1d72a71ad75abfb340aac7e549117e29a08d",
	"0xd70fF30a6fbF2D968D05a44D758961dD29c5cA8D": "c419e539d781332ed6dbd0b6f6690b5e21980964d9fe1bd3eb9e59541d8ba2f4",
	"0x0474C30aB5525553B8Db6eA0B3Da24eC355E027E": "7ad766b3cb259d2ebb8505b51be4164bcac0e74ace5087f9859b265bb14d19b9",
	"0x73769E89a7b8cD0fE3aa6AE54F32CA0aFD16b5DB": "a439b5af1a6652c94bd3f05ea52d42f9be73f4cb899d46e62291c0d393e1ed63",
	"0x1558381335D70c2B97fDf76bDA1caaE5Bc8dA767": "dca5eb483d1b1325b8f74bc24f32ac2909c641b589a608d5054a21e711e5adfa",
	"0x884CF7D856b7572d52200A816FB07a213e5E729d": "88b535cf691ed1993bb030a6964f92fb7df6d3e5c1a9258440f4573eff9b4186",
	"0x418957DB47CaC2936832Ea712C12433f05E44552": "4ba5950688434f83d2a67ad5c1b81a7ad6743630d21a5b74e0f4ba32f870d40d",
	"0x78E794Fd5AaE9d76AE97AA04B05352fAFaE9C397": "64aba5dc8c1d9adfcfe58e0f58d1d19bbc70c4f52e164717b667b1ba1f1fa7b3",
	"0x0826cd0B2E2111e24158D8c3D5556c9e4b8037F1": "a349711a4db7585e9a212e2a643af96a953628bd4ea3bcca7d919ee9fc322d5f",
	"0xF93dC6002227D124D6d037d4F3392c18c04f21e4": "f8a8a01c2857b620e1a550b75dc977975493652aa118ab2712926ca83326b243",
	"0x9D7Caa289428c4C690EC6144822368A20e984067": "1e9e3619c163bba3e5e18822c1b57210591722e9e0bb6f64c7479f6b582504e9",
	"0x22D5Ced4C05F04687CB5b3FEdF18079BB3636592": "6f8800f09f9f22e51124aae040ed03a713e485d6a9563bc8070d64bf87672f0d",
	"0x50dee5466CEAc1e1b880a315C224d62041c6ffD8": "fa06938a1456c24f9b9a867e015c87b66c23735e1948ab7def20b869d1790660",
	"0xb0956d4ef6CCADBc75c83234826E5ceA24d41568": "19fdb73417709e22cda2ddb4a893ea01a6cd6a6b3ecf418c2be415441fea0e07",
	"0xfa5eDF2d1D51B1c140e537f3f11e32232dB34e98": "5ce2fa84847b837017babffb1204aabd097244ce0f3a9d03b1a2ec846eb75ad8",
	"0xdC9148b6e4E23A46dD76A8bBCd9029d244a4Eb4a": "dd99f8377bc7a507ec3a6f8f555a19dade1a37731c38267edcebdd812cf9f295",
	"0x748019daa742DEC4034009D9E368ccE45b6efd6D": "1f4038ec10c80fda627463cb639f61920610d539c09dee90ac26532a48694be0",
	"0xA2c8992Ba761B69e0B1e3cCA686200D7685d9229": "e1348e681d5bac8f05ee3e4138cc5412448fdd8f37483887d4ceb16c7eeb85c1",
	"0xb961E01570BDF2Ac2dE9Cd7518bC027ce28C4932": "7876264be3aeebb4f5f1ffb92affe91e480fadb70a21f636c1ee82a5beae0124",
	"0x8E4D802Ca42AaCE90123a5CE69DD18Ab18BFE966": "81d180433bf5748208e85eed034a2f8a952bdd006fdf2acd6fe4874920a857e5",
	"0x1093AD441C85F861118Ad0230A2D2DF51555dd0E": "928effe7a18efffd8ef497a0dabbeb8f6f45ed317a0d7b5818edec414c3493e0",
	"0xFfe2566580F08b6E42B1Cd713c1887158153DfE7": "532f22f41f34014ef5e7db68e63364d46d42ab98fdba1b0058043cb4e38fb74b",
	"0xa36d45ecDaf8f3A922532b9ca7DFDC428c9357FA": "d03e4701d5cd1867e7a528a2ce9f76c819a9031e84138c046b3bd63f2e407238",
	"0xEe727280f574fc81FaF05D153CcEFf4af1D08e83": "f51042e1b9631915e2d3e5c6662497321a817f5e14fd59f982112ccadc786ad3",
}

var GenesisAddrKeys9 = map[string]string{
	"0x04b057136F3a1e1131434592E5d6209891726984": "2cd80fb9e12fc746b9fc07d8fcc9a08cc4ca98c49cf36bac6c405ed4b224b0b4",
	"0x45dD0B611Cacb95dEBE7be429b872D92953c7301": "b85afc0b7324e829a2bbf9ebd0162cc9261ec56d4cf48aa2cb8fbddf65851e52",
	"0xF8b8cEe574dcfAa87Bf356C9994E2f7087517165": "7492f4880e5258cbb548730ddad7c1c87e93c6c8f73c6f54dfa4a46cbe81720f",
	"0x6c32b5Ff6cc20113767e18b69A6d23B2965E3Dc6": "d991753f7277d1ffb0ce1e90fa1b223e0b93d62ed1559cc0a05d81ef02429d75",
	"0xD2D94924a6D99f58CEBA21470101aa2537a9167a": "147464703b6351d2fb7bfae24b10cf4ff14f63ff020765fe28ebd04e6869599c",
	"0x19Bd5140df2411F6485033d6F33602B6200671dB": "268f76aa981c1b54555054b0db519f8e8eba438e8fd5a6d43a0495eafd9a7a27",
	"0x9cA23b2af9Fa928F56E1375c09Ec7d0CcbD8617f": "ba76468ace8e2063b6a14633cd1ca408769c19906277bd509254d2c35aad52bd",
	"0xB7074f96497973ba2b3D598c24bE3d568fA1e3C0": "5f167edbdfb9e523a31cbe3d765252a05edc3589a75840015315437b24c46604",
	"0x14A45C9120A3A0a8B32d34bb109624658917b71C": "a43d6876b07322f881f1b6b94ac0f4ac52c5d05a58135fd1b9a77c07077ac9b7",
	"0x1e2069B8F864F3db3878e799a3F40Df9aad6e9e6": "bb31a21b718cd205b650b0f8e4a5a1d5fe9cb57ce6e891b8a0a549684833e3ff",
	"0x795C1Ea3c24496fFa41Ada71411d6CC1dbe327D1": "7c693c8ef61229d0db902047460c42f57deda6478f098e19c7e4aae634de7e41",
	"0x67603Af26D11E351f9B1BdbD34800969BDB505b8": "590b9648d604caf8d68d22274d16ff20cb9fdba4de47f7a308b9d51fdbde5cb3",
	"0x302dFB4a3E79Fb5e52308E57d95B69824141F2b8": "90a59bc284b2390fbeeac1701c75b4838aee899d5a0be15935c07c221966a564",
	"0x59660638Bb4FdAD4062E6329AA9b7CdE11b01Db4": "2f70aee986165688e64647a97801838e7a29e053458640ac594aea7317deb3af",
	"0xf25128F38C10fD96868de05A4f4c5668141452b0": "c4d17fd2cd2ba2b6eb44031b5ff431a0b4be00ce42ea7feadc3e3ea5270a80a8",
	"0xE20aDB0AfE4296977868aE7F54f93F96b48D2de2": "e805025f25fa4c074522e8381ef417c046e4849de7db6f2a5ccaddd0e1b778e5",
	"0x24D9D2e946b8905977d707A5A482Ac41d929C57c": "ea80cd9ee08510f2c29bf51f778cc06628da9032fc6046122ab87c80adae73ad",
	"0x85f488313dDD3F0b23F6F64CDcC08eC044b16B9E": "e4fe9f8a00a0d05c0007d575b563d2159cd77fe8d07ddd6aaf8866830bf0bdbe",
	"0xbEF8B2c7fB2b1eC8dE91aD525C0B32f43bdC5FaC": "9ea0a70fa9162b877e67269878a8c29ddfc83b3c543c9750e6a42e3d0ea51af9",
	"0x3c7d65C29e0E31cb77E5F018ab9402B38FAa36d2": "e95c6ea158ec6cf34cb39fefff837d601c190d4d64b673a48689ad50910f733c",
	"0xd2Df380C8648add1Bdadc3884dc234e918070De4": "e693164261f11281f42c0fede1b9f15fd2322366bc88457ab3248b021632f44c",
	"0x1FD53eF5E5F22aD745245f3B16541AEc60cBD42e": "c9547eca9e5c57f931cfce23e88c8f835e119115d3b66e8a4d232a4c5615676f",
	"0xfB8Dd31052B658e36A53050662a40c5a26cbe8f1": "4e6afd7eeb16d498c1e2d8b699141f8be1482419d3dbccf03a1d27465d8f47b8",
	"0x82471E29EFD8fd6D7328e6366e8EF6240BF48D2D": "6ad021bd3461c601914abf91dcdb1e32566cc69466f7173aa2d2df420639e133",
	"0xa2F84326C9a8cc2e4918b2d062fe358Ace222edf": "cfa2a0d189e92a6c0c779a0af21633f52227e88a976d95cab44e998aba7f0ab0",
	"0x114B4718aeB64209262bc2b75c4Cc88Ef7ca6885": "284534f9d3f421a85b6c6bd3c036921668c9fd6299ed8ea1e94abdee6c976421",
	"0xa9b690725Bf6a4Bc834f8fa32D7dB4D254b11419": "dca082d1a7ff070e116efb9a3cdfcc6a746f8d8bf4eeb2ec27cc41c1c40b7143",
	"0xbC8E41D821Da6d9532131c5858E9b79aC195b618": "d07714209a3104a797d653e9f207ee13ca0b7836a9add99e18e44eabf1979fda",
	"0x6De82deCB01041a91337F2A2BfEE4BBFf7C2F960": "bdf9efac3f40c4feab9f415b0cf8301cd46e7dd32960660498a97745864f67e6",
	"0xC28537992B07fFd4B27B182cE984065552F3e078": "1aec162bcf99e7145468a7ddd4849ca18e49c0904a099fd6e416cbd3a6da7a44",
	"0x2dc01055524438143Ca84b90b5CF66CC4B1D2a14": "915d8b34edc00e77ea57749032e9d720f5aebeeb61af1e8fffa85f0ed3bd87c5",
	"0x82aD02241549e79681991FE366178F1285757d0C": "902e1e27b8e0bde2aefe305cc83e18c93ae2ec42d49a92ad01f039a23beace1d",
	"0x9ef64d8E076E16441819aB9818651e060f1968E5": "ebac0bd9c2bc8cb73417ae32adfadb9fb991d101ad108d7b8af70005d4c1491b",
	"0x75E097BB62EF2C601d0095a92A5Fd48e120d41C2": "c86714f26e44bf5285bd3fe3bc4be8d7af1a41a3170ecfa30bd4110ea905becd",
	"0x8fE3F91b8963a6C4D128B4eEbcf63D8A83A02d80": "d4aa6f11f48aaebf9caf3f477a9f28805d837811d4498e5d45952371b2f52447",
	"0x1744467d9385EC219B95cCd0Ea9034225457540c": "a54e2e8eb53b05e8a7e07f67a9e8933e01aef3b1e39751b6240ca3bf9b04a580",
	"0xbCA43b9044649E8f181cA07452E950a148348E15": "c3a39045b6f4745983be58465d19d428d412f2ed9f4b84c3840d348df8989e79",
	"0x8B060991cF5FD334AdcD4eAbd94595Bb6f08Cf28": "fc2039b333c404034b05da876de33c95208ae3a3118b4195797fdd6c60769e4d",
	"0xc52dcb6dF8C5125706F966F2f10664Ce123b0486": "5e57fe25a6c6ee58cc363d59885d7b4b9b5cfc7faa1d23d125803636ed25e021",
	"0x38F64E3915aC3aeb8a0bCE9899A73D338801b8D9": "eb6cb557a1838ca7a8de59e3881918c8876f56930f987a0adba30e4c2f8ad08e",
	"0xaCdb0A8A95393B54Bbc1bBc4a1FC35C8BD003e4a": "8df2981836031ac4da47cc465b081622a46980a6a6196d6c3d985e61ed9e3636",
	"0xdF0c449bB6EdD6aCE64201bCd437d8767d2Eb8D8": "4420f68d151cbc9621824c99c20cae7e1492e3ce23096b0469c27f89f7841977",
	"0x7087252fbC35578628F7cdF065132F6f3e0f0eFE": "608819d72a47a1be3e76083531fada7bf3c83bbc57cfe5821d130eadf18bbd14",
	"0x081f36794e56a0f8Af21a34323b1eEB561db46A9": "89c30ef896099506a3a85226e3488202df5cc235730c7f9bd16b4038e08e0946",
	"0xad02ab314894DB437C2eaB581e71eafa7A2179f2": "b3644d071450bc2dbdd4cd58598e5cb1e444fbf6575f76e3fb5f14bc5a78ede8",
	"0x42655333489a5DFE6251dAc994Fecb5a1700a8Ef": "b89ca7a6cd0377fd5a2fdbf08962d004b64c4afc5d544e7803eb7358d64e3dc9",
	"0xFF448DdF7E5C003A8572C4E561c06b1D13b5c1db": "eb5dd1a84963b02d9fea6ef0875799a70b5ff728b64d34a11eaa96a47d0a6305",
	"0x216F58b2B721B002F726Cf6702dFe4d06e152989": "67348b95e069bd88e183ab42cc8e9020910b43e690444d4afb68fa644139874b",
	"0xB68C77c5b335543DF4B9099B094ceDCba1848BD5": "2c66eb09199af24a0310e93237a21c5ab2d1253af48b8dc3e4736b4fdbd7823f",
	"0x961631d0c17211aFDd6d7A58ec331D812656cf7A": "a0f5fa8d27795d7afe202e17a8fdf35a77b30ad53e03936731f3a335d49100fb",
}

var GenesisAddrKeys10 = map[string]string{
	"0x9e6F688059c8583B0699075f20E5774a865Ad669": "4568e2ccdb57bf8ebf843d234f40446317cc2f3e470c68b70656be2a582ff044",
	"0xcdD89551A82101d69A20fD267d00eD8b7b918A5d": "2847fb90098fbee0c98f05b3f068471fb7dfcb22fb24e55feca9dc7736f573dc",
	"0x80EcBB528f70cD607Cfa94865aFBA50b4c36cDfB": "8fef0dbf4837ce793554cef0ac1376118917b6fc8e5cbdddb110df6472760d73",
	"0x81D245E903748F87eD8A0FA0f786D0859b4a8208": "985319c73f35c7f56168c9a5368c6f38fd7899ff03fb53640b24321f9ba6b4ca",
	"0xeA12e5B40e3C733DdB13b73147B4B58b4D1080A0": "bc56be269b24a0989736b21b465764fc2c8af0c0e346656cdca16b06a1494171",
	"0xaD0bfB99473606DA7513f8BaE58ECFa47bb1752e": "e56ee5bfb9a8e8831ea47c9950d379d300c79edcac490150c583b1665854b68e",
	"0x0e8ea75DE08A428ABaD52a4e738A6023DFae29cA": "9044eb33c6a4211d15d5711dd5b548ef4fdd15915d0589f5091be8b22d8f27c2",
	"0x3746d8B917081019Be70c0A19cef8E8Ac344B37f": "deaf92f65f2cbcac375bc2cab9ef18b1a60521a11ecbd20d6e107ad9b45a4a60",
	"0x163923a6A51a8c78D5623C84Ce303AeB8C7C016a": "5559eeb00ea5dcf80871516cc7a6f4b1af3671680c11a8d2ca16d873e497a440",
	"0x9A0DBe2A02039EE25545Ef029A4DBb0e890D6278": "6e9564c88aa96379a6828dca42c898dbf5491a106ef3695dc177246dbbd8436d",
	"0x09720D4ce1eD9BF59AE9Af4EC6A41f87DE3fe2c5": "a5aa4aa7c422406f4a04e20dbdf9df7d351b3e9e5fa1b29b54f1e91637672b15",
	"0x04e5174bdc99713E1EB24d0Dd648Fe7372743a0D": "ecb6d6132b3e9ae12accb96b08a29343d76faf971d9d5ffde3276ac1b29b1789",
	"0x4d6C9FFEBcdD3f34D42537887D7fC81c4a3f5052": "1b38c3247e37bd5be6bfc889a878fe974eea0a444485b990bbcefec493941276",
	"0x9F4bDF10291597b2AD2B0dCFFd90834C8227df0f": "32711ab7efd2e0eae7f6f5a5da43729bdc0055f8bd04f70cefab8ecfc83ebe75",
	"0xAA8564b772e6357bf4016e573aA409DBCFbF9e68": "6eba8e740fdc53b429db87cf9bd25b5fde5cca247cb2ff8679c7d4e41253e743",
	"0x085d50BB4a54C018Dc39efcD6c626f390716856D": "6b80b23ba08f0cb416e7b66dc54964c76bd56432d2655a88bb203d251fa1bd14",
	"0xeF24085A18CE48f2A31B3250b837E129c13c645e": "e625c807d29544440b6b47718697916a31811c68c27a07bbec9a9d1fac6fe2bb",
	"0xf4Efa66048dD4339cE3c8bB7767C687711e47be6": "3198d1de9b888f9bdefab1e546295ec1391acc471fe2df0ac4906e01d061ee17",
	"0x1299eA3C6894b8f8b9bc881E5D2c078bDd2E827E": "ae3209034383297923a8d24b39311b8e6fc6675e9bda794d1f0781b952558b1c",
	"0x867902A8134011A32dcc9B00553A76Bea408BdED": "c4d22fdd343554c8a7d59b1691133288dd11edab78d8eb3a834a19e11c779479",
	"0xBDA1F3Cab11F45d971b32064b5091fc78142ED17": "be23681fd85262d6d25b76aa3f24e219b4ff191a1d43bd43f370ab53c768656b",
	"0xDe19f4944716757aF2C9B63598685Cc3Cd4e1F17": "582709c9a764a645b73c010ccb34d2e77022457547c2794587dad7fad1268393",
	"0xDdd3764D26c74fce90CeB5b9E250aAB9D73cb327": "f7f67022ce5880996ae48d35d3e4b561585d327f5839f3c59b2ca4b0821638bf",
	"0xbC24c185Ae574c3d55eE8Cf08B67874A22d71Ef8": "9d67578102851b4158828b5a9434bec8d8d47974220555ffb4b85aecf006b650",
	"0x9d988F8665d13F6E79a7390f34A35FF06e7A0CfA": "a6d4180e12eb3b2bdbd3bdee154a002991215b0a74c5618161bc8d5ce0a28389",
	"0xa7ff5138DbD052c702DAa3F637317ff023Ae0197": "4f880ac95771f96181cfad6db0495d9d02eca5b1641875b73b3f3f6686acf31d",
	"0x3aB9b44Dbf41c11529a2B60772A53e2026581769": "f6afbdfc4b83f7ebd7bdd108b94a53815708b53346b4f1b5e1b141dd4ed994bb",
	"0x82F0463D8728d9928587cD46c4BE225aABe810dB": "1d01d7421d91eb6fc48a88acc7251cac848f957d762aa6262487acc11a4e4047",
	"0x9a141b57C8107755Bab955d1803D886EB8C25494": "63207e72a585950918aa52f8c9c19e1877ff1db22e8e27448aaf521bc2f66568",
	"0x8735FA5555Fe70127DE9e9C3b68333506B8F5D26": "6965b77771d999335a2d2b893ca74478a06462dad1fe6b40c9c9df7ec465952f",
	"0x6a547246886f6A75fdB3558f19B03Ca9e4c11506": "2b279b25da0fe355077bacade8d48a7d26507a4c83c768b3272b1a17e6340dce",
	"0xBC0C5a3b81e739F638Be76a07e21984e93f2989F": "8ee2ed80efab658451ebc0388bef18de6e3572c272b312e222f3c8e9d641f767",
	"0xde0918C68EcE228adb6bCfc0B1280FFE0DF9e58B": "45fda8be126182598c9a9e745d9e54fcc84e610a7c1cf51a8d24b7949f11630c",
	"0x9f8641398c017c586D76d69b55DD478B81f41Dc8": "68df6f08d49fd74e8d3d39e55e3fdda6e531e5a3505e655c551891557407f13a",
	"0x3f5be4E4E5D9C9fA0011A5b8661cF9AC26D592d5": "325dc859e73e848c28cf3dbb09e50c59608724da3a88b43d5104d7b92d3a45ea",
	"0x119b888b00BAa5013d649491e65F60203F24af8e": "9b012502b0e740e1966cdc170681274a139a9f120f12c6fca68041ee5afbc425",
	"0xbAbe3BdF1E1CB5be5015D0d29963B58c85Ae730b": "a92a1c050f26b064ee04b4b537008fd8048641dbb1b61f7f2220c120fe6e77d9",
	"0xf2aE11E787269a8D7a4553A0CcB91865bb505C6a": "8c9d2f95e0acfe823c0e930d52688f051729ed7c63609c5dec077338b2e98659",
	"0x914bB6E5F5FadfE9577544199037129822983912": "691352fa810c8f8659fd81d70ec95ed84346be2b2c338b80e310a029df6caf71",
	"0x8b79E9a62A2cB9fe8a37eA4E1A01c61bBD78aA73": "a1370b03295dd1f8e51075387616d8d63d39ad3e0cb0f042a9de616c561830c9",
	"0x26f27316452E6d10D7C37A53cB09Df242F04162E": "eb7d5ae712baf6760c106b589e5742d4f65ce216c08d361ef791deb9122a2b5b",
	"0x20aEd0b9f2371149ceDE8Bb565e5b5f26E25E45e": "3da377052f2a8166e08e9059e9289f662f28c0c77dfe2436599eff9f13ff5142",
	"0x1C594631Ad3b97b9a111e76d46CCA521A0E47f66": "8d20c051e5c9fda2c9c21eab1b95465396bba169aa61038dcdf0a4d076e3d0e5",
	"0xE7677bd67007E77b49C66C6A0a6bEe75C4177184": "ee54e6092ea425698e7d17713af3957186bd02ed0707c0a42257fb84ba7fd4ff",
	"0x0EC75412B817435b1E7F5D6bA223a7B3ce88d349": "86672119e987613c3bf341061522d58087171ddea7a3312dab5fa5ffb3289b10",
	"0x3783496C758d9C9a97d9118F597B08d822D9d67E": "d570b39647d2a32e5e0829b9d75e2aa1ef54cededd72fd4b5bd5419c6a82ea46",
	"0xa0EfC699Db0c226e9a8eAF11D27Ca1d7Bc815854": "c7619108acc9f29d25ff805ebebee02103502582069a6054603d613a79581dba",
	"0xCd7b57D1ce9E9f3D5a38Eaa4960803B3eDAE489F": "cd457556f16ba63dfdee85b47432b0c44f8b42c96aaadd945e352247b55ea6ea",
	"0x886906c1BF89bD5a5265bc3fccC9C4E053F52050": "c4395b185c2faf69a9e0703b809459d2a72afb7b3e69e48403e8fc3b73b9c8a4",
	"0x7b2A8573243cF7e7F85da21753919DC3d9f38659": "a8c8b43c47f91ee05fe45b64546e5b58ea026eebbc3c835885dbae2b00d83736",
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
