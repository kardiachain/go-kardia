package consensus

import (
	"fmt"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"strings"
)

const maximumGasUsed = uint64(7000000)
var KAI, _ = big.NewInt(0).SetString("100000000000000000", 10) // 10**18

// staticCall calls smc and return result in bytes format
func staticCall(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, statedb *state.StateDB) (result []byte, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, maximumGasUsed)
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}

func call(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, value *big.Int, statedb *state.StateDB) (result []byte, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.Call(sender, to, input, maximumGasUsed, value)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func create(from common.Address, to common.Address, currentHeader *types.Header, chain vm.ChainContext, input []byte, value *big.Int, statedb *state.StateDB) (result []byte, address *common.Address, leftOverGas uint64, err error) {
	ctx := vm.NewKVMContextFromDualNodeCall(from, currentHeader, chain)
	vmenv := kvm.NewKVM(ctx, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, contractAddr, leftOver, err := vmenv.CreateGenesisContract(sender, &to, input, maximumGasUsed, value)
	if err != nil {
		return make([]byte, 0), nil, leftOver, err
	}
	address = &contractAddr
	return ret, address, leftOver, nil
}

func InitGenesisConsensus(bc *blockchain.BlockChain, consensusInfo node.ConsensusInfo) (*types.ValidatorSet, error) {
	st, err := bc.State()
	if err != nil {
		return nil, err
	}
	// get first node owner to be the sender
	sender := consensusInfo.Nodes.GenesisInfo[0].Owner
	// create master smart contract
	if err := createMaster(bc, st, consensusInfo.Master, consensusInfo.MaxValidators, consensusInfo.ConsensusPeriod, sender); err != nil {
		return nil, err
	}
	// add stakers
	if err = addStakers(bc, st, consensusInfo.Master, consensusInfo.Stakers.GenesisInfo, sender); err != nil {
		return nil, err
	}
	// create nodes
	if err := createGenesisNodes(bc, st, consensusInfo.Nodes, consensusInfo.Master.Address); err != nil {
		return nil, err
	}
	// create stakers and stake them
	if err := createGenesisStakers(bc, st, consensusInfo.Stakers, consensusInfo.Master.Address, consensusInfo.MinimumStakes); err != nil {
		return nil, err
	}
	// start collect validators
	if err := CollectValidators(bc, st, consensusInfo.Master, sender); err != nil {
		return nil, err
	}
	// collect validator set
	return CollectValidatorSet(bc, st, consensusInfo.Master, consensusInfo.Nodes.ABI, sender)
}

func createMaster(bc *blockchain.BlockChain, st *state.StateDB, master node.MasterSmartContract, maxValidators uint64, consensusPeriod uint64, sender common.Address) error {
	masterAbi, err := abi.JSON(strings.NewReader(master.ABI))
	if err != nil {
		return err
	}
	input, err := masterAbi.Pack("", consensusPeriod, maxValidators)
	if err != nil {
		return err
	}
	newCode := append(master.ByteCode, input...)
	_, _, _, err = create(sender, master.Address, bc.CurrentHeader(), bc, newCode, master.GenesisAmount, st)
	if err != nil {
		return err
	}
	return nil
}

func addStakers(bc *blockchain.BlockChain, st *state.StateDB, master node.MasterSmartContract, stakers []node.GenesisStakeInfo, sender common.Address) error {
	masterAbi, err := abi.JSON(strings.NewReader(master.ABI))
	if err != nil {
		return err
	}
	for _, staker := range stakers {
		input, err := masterAbi.Pack("addStaker", staker.Address)
		if err != nil {
			return err
		}
		if _, err = call(sender, master.Address, bc.CurrentHeader(), bc, input, big.NewInt(0), st); err != nil {
			return err
		}
	}
	return nil
}

func createGenesisNodes(bc *blockchain.BlockChain, st *state.StateDB, nodes node.Nodes, masterAddress common.Address) error {
	nodeAbi, err := abi.JSON(strings.NewReader(nodes.ABI))
	if err != nil {
		return err
	}
	for _, n := range nodes.GenesisInfo {
		input, err := nodeAbi.Pack("", masterAddress, n.Owner, n.PubKey, n.Name, n.Host, n.Port, n.Reward)
		if err != nil {
			return err
		}
		newCode := append(nodes.ByteCode, input...)
		if _, _, _, err = create(n.Owner, n.Address, bc.CurrentHeader(), bc, newCode, big.NewInt(0), st); err != nil {
			return err
		}
	}
	return nil
}

func createGenesisStakers(bc *blockchain.BlockChain, st *state.StateDB, stakers node.Stakers, masterAddress common.Address, minimumStakes *big.Int) error {
	stakerAbi, err := abi.JSON(strings.NewReader(stakers.ABI))
	if err != nil {
		return err
	}
	for _, staker := range stakers.GenesisInfo {
		input, err := stakerAbi.Pack("", masterAddress, staker.Owner, big.NewInt(int64(staker.LockedPeriod)), minimumStakes)
		if err != nil {
			return err
		}
		newStakerCode := append(stakers.ByteCode, input...)
		if _, _, _, err = create(staker.Owner, staker.Address, bc.CurrentHeader(), bc, newStakerCode, big.NewInt(0), st); err != nil {
			return err
		}
		// stake to staker
		stakeInput, err := stakerAbi.Pack("stake", staker.StakedNode)
		if err != nil {
			return err
		}
		if _, err = call(staker.Owner, staker.Address, bc.CurrentHeader(), bc, stakeInput, staker.StakeAmount, st); err != nil {
			return err
		}
	}
	return nil
}

func CollectValidators(bc *blockchain.BlockChain, st *state.StateDB, master node.MasterSmartContract, sender common.Address) error {
	masterAbi, err := abi.JSON(strings.NewReader(master.ABI))
	if err != nil {
		return nil
	}
	input, err := masterAbi.Pack("collectValidators")
	if err != nil {
		return err
	}
	if _, err = call(sender, master.Address, bc.CurrentHeader(), bc, input, big.NewInt(0), st); err != nil {
		return err
	}
	return nil
}

func CollectValidatorSet(bc *blockchain.BlockChain, st *state.StateDB, master node.MasterSmartContract, nodeAbiStr string, sender common.Address) (*types.ValidatorSet, error) {
	masterAbi, err := abi.JSON(strings.NewReader(master.ABI))
	if err != nil {
		return nil, err	
	}

	nodeAbi, err := abi.JSON(strings.NewReader(nodeAbiStr))
	if err != nil {
		return nil, err
	}

	length, startBlock, endBlock, err := getLatestValidatorsInfo(bc, st, masterAbi, master.Address, sender)
	if err != nil {
		return nil, err
	}

	validators := make([]*types.Validator, 0)
	for i:=uint64(1); i < length; i++ {
		getLatestValidator, err := masterAbi.Pack("GetLatestValidator", i)
		if err != nil {
			return nil, err
		}

		output, err := staticCall(sender, master.Address, bc.CurrentHeader(), bc, getLatestValidator, st)
		if err != nil {
			return nil, err
		}
		
		type validator struct {
			Node common.Address `abi:"node"`
			Owner common.Address `abi:"owner"`
			Stakes *big.Int `abi:"stakes"`
			TotalStaker uint64 `abi:"totalStaker"`
		}
		var val validator
		err = masterAbi.Unpack(&val, "GetLatestValidator", output)
		if err != nil {
			return nil, err
		}

		stakes := calculateVotingPower(val.Stakes)
		if stakes < 0 {
			return nil, fmt.Errorf("invalid stakes")
		}

		// get node info from node address
		getNodeInfo, err := nodeAbi.Pack("getNodeInfo")
		if err != nil {
			return nil, err
		}

		result, err := staticCall(sender, val.Node, bc.CurrentHeader(), bc, getNodeInfo, st)
		if err != nil {
			return nil, err
		}
		type nodeInfo struct {
			Owner common.Address `abi:"owner"`
			NodeId string `abi:"nodeId"`
			NodeName string `abi:"nodeName"`
			IpAddress string `abi:"ipAddress"`
			Port string `abi:"port"`
			RewardPercentage uint16 `abi:"rewardPercentage"`
			Balance *big.Int `abi:"balance"`
		}
		var n nodeInfo
		err = nodeAbi.Unpack(&n, "getNodeInfo", result)
		if err != nil {
			return nil, err
		}

		pubKey, err := crypto.StringToPublicKey(n.NodeId)
		if err != nil {
			return nil, err
		}
		validators = append(validators, types.NewValidator(*pubKey, stakes))
	}
	return types.NewValidatorSet(validators, int64(startBlock), int64(endBlock)), nil
}

func getLatestValidatorsInfo(bc *blockchain.BlockChain, st *state.StateDB, masterAbi abi.ABI, masterAddress, sender common.Address) (uint64, uint64, uint64, error) {
	method := "getLatestValidatorsInfo"
	input, err := masterAbi.Pack(method)
	if err != nil {
		return 0, 0, 0, err
	}
	output, err := staticCall(sender, masterAddress, bc.CurrentHeader(), bc, input, st)
	if err != nil {
		return 0, 0, 0, err
	}
	type getLatestValidatorsInfo struct {
		TotalNodes uint64 `abi:"totalNodes"`
		StartAtBlock uint64 `abi:"startAtBlock"`
		EndAtBlock uint64 `abi:"endAtBlock"`
	}
	var info getLatestValidatorsInfo
	if err = masterAbi.Unpack(&info, method, output); err != nil {
		return 0, 0, 0, err
	}
	return info.TotalNodes, info.StartAtBlock, info.EndAtBlock, nil
}

func calculateVotingPower(amount *big.Int) int64 {
	return amount.Div(amount, KAI).Int64()
}