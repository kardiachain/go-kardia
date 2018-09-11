// Defines default configs used for initializing nodes in dev settings.

package dev

import (
	"bufio"
	"crypto/ecdsa"
	"encoding/csv"
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
	password  = "KardiaChain"
	ChainData = "chaindata"
	DbCache   = 16
	DbHandles = 16
)

var initValue = int64(math.Pow10(15))

// GenesisAccounts are used to initialized accounts in genesis block
var GenesisAccounts = map[string]int64{
	"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
	"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	"0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd": initValue,
	"0x071E8F5ddddd9f2D4B4Bdf8Fc970DFe8d9871c28": initValue,
	"0x94FD535AAB6C01302147Be7819D07817647f7B63": initValue,
	"0xa8073C95521a6Db54f4b5ca31a04773B093e9274": initValue,
	"0xe94517a4f6f45e80CbAaFfBb0b845F4c0FDD7547": initValue,
	"0xBA30505351c17F4c818d94a990eDeD95e166474b": initValue,
	//"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": 100000000,
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"0x36BE7365e6037bD0FDa455DC4d197B07A2002547": 100000000,
}

var GenesisContractAddress = []string {
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
	"0x00000000000000000000000000000000736D6333": "6080604052600436106100775763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630a0306b1811461007c578063323a9243146100a357806344af18e8146100bd5780636e63987d146100d557806386dca334146100ed578063fa8513de14610105575b600080fd5b34801561008857600080fd5b5061009161011a565b60408051918252519081900360200190f35b3480156100af57600080fd5b506100bb600435610139565b005b3480156100c957600080fd5b506100bb600435610154565b3480156100e157600080fd5b506100bb60043561016f565b3480156100f957600080fd5b506100bb60043561017a565b34801561011157600080fd5b50610091610185565b600060015460005411156101315750600154610136565b506000545b90565b60015481111561014857600080fd5b60018054919091039055565b60005481111561016357600080fd5b60008054919091039055565b600180549091019055565b600080549091019055565b60008054600154111561019b5750600054610136565b50600154905600a165627a7a72305820f07bf8b0278729f61585fdeb608ea6ab12a34ae7871ea92bfd2f4199cc5bfd0d0029",
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
		{"constant": false,"inputs": [{"name": "eth","type": "uint256"}],"name": "matchEth","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": false,"inputs": [{"name": "neo","type": "uint256"}],"name": "matchNeo","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": false,"inputs": [{"name": "eth","type": "uint256"}],"name": "removeEth","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": false,"inputs": [{"name": "neo","type": "uint256"}],"name": "removeNeo","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},
		{"constant": true,"inputs": [],"name": "getEthToSend","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "view","type": "function"},
		{"constant": true,"inputs": [],"name": "getNeoToSend","outputs": [{"name": "","type": "uint256"}],"payable": false,"stateMutability": "view","type": "function"}
	]`,
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
	//"0x212a83C0D7Db5C526303f873D9CeaA32382b55D0": "b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",

	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d": "0x36BE7365e6037bD0FDa455DC4d197B07A2002547",
}

var nodes = []node{
	{"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06", 100, "enode://724fbdc7067814bdd60315d836f28175ff9c72e4e1d86513a2b578f9cd769e688d6337550778b89e4861a42580613f1f1dec23f17f7a1627aa99104cc4204eb1@[::]:3000"},
	{"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79", 100, "enode://b5997edd19d36bd5a1afa701563ca5b505bacb55c840ba35aa961c46af6484c4a5069c29062fa1c9fe6754ebdb42f6b16a549a2832d080f5faf13b42993d31b8@[::]:3001"},
	{"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb", 100, "enode://a840f4fa933e741f0c2f4dcafc6e0bbd05a364fa7ee61a312dcefbfb9a7081a5f3028419e6d94acfd95af2e3a8c884bf48d286f3015d1f7db7a4ba6030b3e66a@[::]:3002"},
	{"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe", 100, "enode://e00744773f3f3d641df1df591eb1212a80c6ecc49a1f1d43ee8e30e602259726274fb1817b346b7a8fc61db5cfb531c6827044cc4c7f6d2b67dda03c5cee8b8e@[::]:3003"},
	{"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a", 100, "enode://2773b8f006193853112e529dbab168e4c62da80beebfa151b82547ca8ba54a5cef94b30ecd3c4dd5bf54100fc642062dcec87ae98c08e504e5c985038f994325@[::]:3004"},
	{"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265", 100, "enode://754d2ab0c56c963a64f08a11d4c5d92436e81b81a1422959bc0a5ae802b099499188646a6e6ecc3878aa0a7e4edf599fc7101d4556ae4f818c7de05bd8e810b7@[::]:3005"},
	{"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007", 100, "enode://ebdfa0502b2e095d493780f50024fe6746f52893d986c74dfedb44c01f834fc68f1373df7c2035feb8542dfa86d27bbdc71c0f065545c135baf55a6a84b24870@[::]:3006"},
	{"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7", 100, "enode://ca299c6ba014473c3ac53d3e9fa92f60dc34d838095c07fff2aab350439374bc3dbfe4411757e984c6b191868898755a328c3e97fc8abb37f6eed4b73ca7f67b@[::]:3007"},
	//{"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67", 100, "enode://e9f1abb546000dbfba59894610053e4bf337ef1db64253e5d30162c8e17ff49b9980150363e09ddeac1db6e23b4ed683ad3c1bda1ad5886112b18c47f7ad9eae@[::]:3008"},
	// TODO(namdoh): Re-enable after parsing node index fixed in main.go
	//{"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d", 100, ""},
}

func CreateDevEnvironmentConfig() *DevEnvironmentConfig {
	var devEnv DevEnvironmentConfig
	devEnv.proposalIndex = 0 // Default to 0-th node as the proposer.
	devEnv.DevNodeSet = make([]DevNodeConfig, len(nodes))
	for i, n := range nodes {
		privKey, _ := crypto.ToECDSA([]byte(n.key[:32]))
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

func (devEnv *DevEnvironmentConfig) GetValidatorSet(numVal int) *types.ValidatorSet {
	if numVal < 0 || numVal >= devEnv.GetNodeSize() {
		log.Error(fmt.Sprintf("Number of validator must be within %v and %v", 0, devEnv.GetNodeSize()))
	}
	validators := make([]*types.Validator, numVal)
	for i := 0; i < numVal; i++ {
		node := devEnv.DevNodeSet[i]
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
	panic("impossible failure")
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
