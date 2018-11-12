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

// Defines default configs used for initializing nodes in dev settings.

package dev

import (
	"bufio"
	"crypto/ecdsa"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

const (
	// GenesisAccount used for matchEth tx
	MockKardiaAccountForMatchEthTx = "0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28"
	// GenesisAccount used for matchEth tx
	MockSmartContractCallSenderAccount = "0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5"
)

type DevNodeConfig struct {
	PrivKey     *ecdsa.PrivateKey
	VotingPower int64
	NodeID      string
}

type DevEnvironmentConfig struct {
	DevNodeSet []DevNodeConfig

	proposalIndex  int
	VotingStrategy map[VoteTurn]int
}

type node struct {
	key         string
	votingPower int64
	nodeID      string
}

type VoteTurn struct {
	Height   int
	Round    int
	VoteType int
}

type account struct {
	address string
	balance int64
}

const (
	// password is used to get keystore
	password = "KardiaChain"
)

var IsUsingNeoTestNet = true
var NeoSubmitTxUrl = "http://35.240.175.184:5000"
var NeoCheckTxUrl = "http://35.240.175.184:4000/api/main_net/v1/get_transaction/"
var TestnetNeoSubmitUrl = "http://35.185.187.119:5000"
var TestnetNeoCheckTxUrl = "https://neoscan-testnet.io/api/test_net/v1/get_transaction/"
var NeoReceiverAddress = "AaXPGsJhyRb55r8tREPWWNcaTHq4iiTFAH"
var initValue = int64(math.Pow10(15))

// GenesisAccounts are used to initialized accounts in genesis block
var GenesisAccounts = map[string]int64{
	// TODO(kiendn): These addresses are same of node address. Change to another set.
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": initValue,
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": initValue,
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": initValue,
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": initValue,
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": initValue,
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": initValue,
	"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": initValue,
	"0x8dB7cF1823fcfa6e9E2063F983b3B96A48EEd5a4": initValue,
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"0x36BE7365e6037bD0FDa455DC4d197B07A2002547": 100000000,
}

var GenesisContractAddress = []string{
	"0x00000000000000000000000000000000736D6332",
	"0x00000000000000000000000000000000736D6331",
	"0x00000000000000000000000000000000736D6333",
}

// RawByteCode used for creating simple counter contract
var RawByteCode = "608060405234801561001057600080fd5b50610108806100206000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806324b8ba5f14604e5780636d4ce63c14607b575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060a9565b005b348015608657600080fd5b50608d60c6565b604051808260ff1660ff16815260200191505060405180910390f35b806000806101000a81548160ff021916908360ff16021790555050565b60008060009054906101000a900460ff169050905600a165627a7a7230582083f88bef40b78ed8ab5f620a7a1fb7953640a541335c5c352ff0877be0ecd0c60029"

// GenesisContract are used to initialize contract in genesis block
var GenesisContracts = map[string]string{
	//"0x00000000000000000000000000000000736d6331": "60806040526004361060485763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166324b8ba5f8114604d5780636d4ce63c146067575b600080fd5b348015605857600080fd5b50606560ff60043516608f565b005b348015607257600080fd5b50607960a5565b6040805160ff9092168252519081900360200190f35b6000805460ff191660ff92909216919091179055565b60005460ff16905600a165627a7a723058206cc1a54f543612d04d3f16b0bbb49e9ded9ccf6d47f7789fe3577260346ed44d0029",
	// Simple voting contract bytecode in genesis block, source code in smc/Ballot.sol
	"0x00000000000000000000000000000000736D6332": "608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063124474a71461005c578063609ff1bd146100a0578063b3f98adc146100d1575b600080fd5b34801561006857600080fd5b5061008a600480360381019080803560ff169060200190929190505050610101565b6040518082815260200191505060405180910390f35b3480156100ac57600080fd5b506100b5610138565b604051808260ff1660ff16815260200191505060405180910390f35b3480156100dd57600080fd5b506100ff600480360381019080803560ff16906020019092919050505061019e565b005b600060048260ff161015156101195760009050610133565b60018260ff1660048110151561012b57fe5b016000015490505b919050565b6000806000809150600090505b60048160ff161015610199578160018260ff1660048110151561016457fe5b0160000154111561018c5760018160ff1660048110151561018157fe5b016000015491508092505b8080600101915050610145565b505090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060000160009054906101000a900460ff1680610201575060048260ff1610155b1561020b5761026a565b60018160000160006101000a81548160ff021916908315150217905550818160000160016101000a81548160ff021916908360ff1602179055506001808360ff1660048110151561025857fe5b01600001600082825401925050819055505b50505600a165627a7a72305820c93a970449b32fe53b59e0ed7cfeda5d52acafd2d1bdd3f2f67093f076acf1c60029",
	// Counter contract bytecode in genesis block, source code in smc/SimpleCounter.sol
	"0x00000000000000000000000000000000736D6331": "6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806324b8ba5f14604e5780636d4ce63c14607b575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060a9565b005b348015608657600080fd5b50608d60c6565b604051808260ff1660ff16815260200191505060405180910390f35b806000806101000a81548160ff021916908360ff16021790555050565b60008060009054906101000a900460ff169050905600a165627a7a7230582083f88bef40b78ed8ab5f620a7a1fb7953640a541335c5c352ff0877be0ecd0c60029",
	// Exchange master contract bytecode in genesis block, source code in smc/Exchange.sol
	"0x00000000000000000000000000000000736D6333": "60806040526004361061008d5763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630a0306b18114610092578063323a9243146100b95780633c3c9c23146100d357806344af18e8146100e8578063613d03af146101005780636e63987d1461011557806386dca3341461012d578063fa8513de14610145575b600080fd5b34801561009e57600080fd5b506100a761015a565b60408051918252519081900360200190f35b3480156100c557600080fd5b506100d1600435610179565b005b3480156100df57600080fd5b506100a7610194565b3480156100f457600080fd5b506100d160043561019a565b34801561010c57600080fd5b506100a76101b5565b34801561012157600080fd5b506100d16004356101bb565b34801561013957600080fd5b506100d16004356101c6565b34801561015157600080fd5b506100a76101d1565b600060015460005411156101715750600154610176565b506000545b90565b60015481111561018857600080fd5b60018054919091039055565b60005481565b6000548111156101a957600080fd5b60008054919091039055565b60015481565b600180549091019055565b600080549091019055565b6000805460015411156101e75750600054610176565b50600154905600a165627a7a723058203f7b9ba72392daf2bb6f8a91c0d4a8a3dcd58decc81ffc4fd90951f41cb9490c0029",
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
}

//  GenesisAddrKeys maps genesis account addresses to private keys.
var GenesisAddrKeys = map[string]string{
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
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d": "0x36BE7365e6037bD0FDa455DC4d197B07A2002547",
}

var nodes = []node{
	{"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06", 100, "enode://7a86e2b7628c76fcae76a8b37025cba698a289a44102c5c021594b5c9fce33072ee7ef992f5e018dc44b98fa11fec53824d79015747e8ac474f4ee15b7fbe860@[::]:3000"},
	{"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79", 100, "enode://660889e39b37ade58f789933954123e56d6498986a0cd9ca63d223e866d5521aaedc9e5298e2f4828a5c90f4c58fb24e19613a462ca0210dd962821794f630f0@[::]:3001"},
	{"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb", 100, "enode://2e61f57201ec804f9d5298c4665844fd077a2516cd33eccea48f7bdf93de5182da4f57dc7b4d8870e5e291c179c05ff04100718b49184f64a7c0d40cc66343da@[::]:3002"},
	{"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe", 100, "enode://fc41a71d7a74d8665dbcc0f48c9a601e30b714ed50647669ef52c03f7123f2ae078dcaa36389e2636e1055f5f60fdf38d89a226ee84234f006b333cad2d2bcee@[::]:3003"},
	{"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a", 100, "enode://ebf46faca754fc90716d665e6c6feb206ca437c9e5f16690e690513b302935053a9d722b88d2ab0b972f46448f3a53378bf5cfe01b8373af2e54197b17617e1c@[::]:3004"},
	{"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265", 100, "enode://80c4fbf65122d817d3808afcb683fc66d9f9e19b476ea0ee3f757dca5cd18316ecb8999bfea4e9a5acc9968504cb919997a5c1ab623c5c533cb662291149b0a3@[::]:3005"},
	{"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007", 100, "enode://5d7ed8131916b10ea545a559abe464307109a3d62ddbe19c368988bbdb1dd2330b6f3bbb479d0bdd79ef360d7d9175008d90f7d51122969210793e8a752cecd6@[::]:3006"},
	{"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7", 100, "enode://7ecd4ea1bf4efa34dac41a16d7ccd14e23d3993dd3f0a54d722ee76d170718adba7f246c082bada922c875ffaaa4618e307b68e44c2847d4f4e3b767884c02b7@[::]:3007"},
	{"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67", 100, "enode://4857f792ef779c511f6d7643f0991409f77e41124ced14385217535641315f5dc9927e7301ffd7afc7ae8025663e17f593306adf7d3ffac7c6aa625c250de0d5@[::]:3008"},
	{"0cf7ae0332a891044659ace49a0732fa07c2872b4aef479945501f385a23e689", 100, "enode://ad67c2502fc2723f2dcf25a140744382eb3e4e50d7e4dd910c423f7aa4fe0fbbcc2207d22ef6edf469dd6fbea73efa8d87b4b876a0d6e386c4e00b6a51c2a3f8@[::]:3009"},
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//{"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d", 100, ""},
}

func CreateDevEnvironmentConfig() *DevEnvironmentConfig {
	var devEnv DevEnvironmentConfig
	devEnv.proposalIndex = 0 // Default to 0-th node as the proposer.
	devEnv.DevNodeSet = make([]DevNodeConfig, len(nodes))
	for i, n := range nodes {
		pkByte, err := hex.DecodeString(n.key)
		if err != nil {
			continue
		}
		privKey, _ := crypto.ToECDSA(pkByte)
		devEnv.DevNodeSet[i].PrivKey = privKey
		devEnv.DevNodeSet[i].VotingPower = n.votingPower
		devEnv.DevNodeSet[i].NodeID = n.nodeID
	}

	return &devEnv
}

func (devEnv *DevEnvironmentConfig) SetVotingStrategy(votingStrategy string) {
	if strings.HasSuffix(votingStrategy, "csv") {
		devEnv.VotingStrategy = map[VoteTurn]int{}
		csvFile, _ := os.Open(votingStrategy)
		reader := csv.NewReader(bufio.NewReader(csvFile))

		for {
			line, error := reader.Read()
			if error == io.EOF {
				break
			} else if error != nil {
				log.Error("error", error)
			}
			var height, _ = strconv.Atoi(line[0])
			var round, _ = strconv.Atoi(line[1])
			var voteType, _ = strconv.Atoi(line[2])
			var result, _ = strconv.Atoi(line[3])

			var _, ok = devEnv.GetScriptedVote(height, round, voteType)
			if ok {
				log.Error(fmt.Sprintf("VoteTurn already exists with height = %v, round = %v, voteType = %v", height, round, voteType))
			} else {
				devEnv.VotingStrategy[VoteTurn{height, round, voteType}] = result
			}
		}
	}
}

func (devEnv *DevEnvironmentConfig) GetScriptedVote(height int, round int, voteType int) (int, bool) {
	if val, ok := devEnv.VotingStrategy[VoteTurn{height, round, voteType}]; ok {
		return val, ok
	}
	return 0, false
}

func (devEnv *DevEnvironmentConfig) SetProposerIndex(index int) {
	if index < 0 || index >= devEnv.GetNodeSize() {
		log.Error(fmt.Sprintf("Proposer index must be within %v and %v", 0, devEnv.GetNodeSize()))
	}
	devEnv.proposalIndex = index
}

func (devEnv *DevEnvironmentConfig) GetDevNodeConfig(index int) *DevNodeConfig {
	return &devEnv.DevNodeSet[index]
}

func (devEnv *DevEnvironmentConfig) GetNodeSize() int {
	return len(devEnv.DevNodeSet)
}

// GetValidatorSetByIndex takes an array of indexes of validators and returns an array of validators with the order respectively to index of input
func (devEnv *DevEnvironmentConfig) GetValidatorSetByIndex(valIndexes []int) *types.ValidatorSet {
	if len(valIndexes) > devEnv.GetNodeSize() {
		log.Error(fmt.Sprintf("Number of validators must be within %v and %v", 1, devEnv.GetNodeSize()))
	}
	validators := make([]*types.Validator, len(valIndexes))
	for i := 0; i < len(valIndexes); i++ {
		if valIndexes[i] < 0 || valIndexes[i] >= devEnv.GetNodeSize() {
			log.Error(fmt.Sprintf("Value of validator must be within %v and %v", 1, devEnv.GetNodeSize()))
		}
		node := devEnv.DevNodeSet[valIndexes[i]]
		validators[i] = types.NewValidator(node.PrivKey.PublicKey, node.VotingPower)
	}

	validatorSet := types.NewValidatorSet(validators)
	validatorSet.TurnOnKeepSameProposer()
	validatorSet.SetProposer(validators[devEnv.proposalIndex])
	return validatorSet
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
func (devEnv *DevEnvironmentConfig) GetRawBytecode() string {
	return RawByteCode
}
