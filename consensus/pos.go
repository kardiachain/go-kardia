package consensus

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/pos"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"strings"
)

var KAI, _ = big.NewInt(0).SetString("100000000000000000", 10) // 10**18
type (
	validator struct {
		Node common.Address `abi:"node"`
		Owner common.Address `abi:"owner"`
		Stakes *big.Int `abi:"stakes"`
		TotalStaker uint64 `abi:"totalStaker"`
	}
	latestValidatorsInfo struct {
		TotalNodes uint64 `abi:"totalNodes"`
		StartAtBlock uint64 `abi:"startAtBlock"`
		EndAtBlock uint64 `abi:"endAtBlock"`
	}
	nodeInfo struct {
		Owner common.Address `abi:"owner"`
		NodeId string `abi:"nodeId"`
		NodeName string `abi:"nodeName"`
		IpAddress string `abi:"ipAddress"`
		Port string `abi:"port"`
		RewardPercentage uint16 `abi:"rewardPercentage"`
		Balance *big.Int `abi:"balance"`
	}
)

func newGenesisVM(from common.Address, gasLimit uint64, st base.StateDB) *kvm.KVM {
	ctx := kvm.NewGenesisKVMContext(from, gasLimit)
	return kvm.NewKVM(ctx, st, kvm.Config{})
}

func InitGenesisConsensus(st *state.StateDB, gasLimit uint64, consensusInfo pos.ConsensusInfo) error {
	var err error
	// get first node owner to be the sender
	sender := consensusInfo.Nodes.GenesisInfo[0].Owner
	// create master smart contract
	if err = createMaster(gasLimit, st, consensusInfo.Master, consensusInfo.MaxValidators, consensusInfo.ConsensusPeriod, sender); err != nil {
		return err
	}
	// add stakers
	if err = addStakers(gasLimit, st, consensusInfo.Master, consensusInfo.Stakers.GenesisInfo, sender); err != nil {
		return err
	}
	// create nodes
	if err = createGenesisNodes(gasLimit, st, consensusInfo.Nodes, consensusInfo.Master.Address); err != nil {
		return err
	}
	// create stakers and stake them
	if err = createGenesisStakers(gasLimit, st, consensusInfo.Stakers, consensusInfo.Master.Address, consensusInfo.MinimumStakes); err != nil {
		return err
	}
	// start collect validators
	return CollectValidators(gasLimit, st, consensusInfo.Master, sender)
}

func createMaster(gasLimit uint64, st *state.StateDB, master pos.MasterSmartContract, maxValidators uint64, consensusPeriod uint64, sender common.Address) error {
	var (
		masterAbi abi.ABI
		err error
		input []byte
	)
	vm := newGenesisVM(sender, gasLimit, st)
	if masterAbi, err = abi.JSON(strings.NewReader(master.ABI)); err != nil {
		return err
	}
	if input, err = masterAbi.Pack("", consensusPeriod, maxValidators); err != nil {
		return err
	}
	newCode := append(master.ByteCode, input...)
	if _, _, _, err = kvm.InternalCreate(vm, master.Address, newCode, master.GenesisAmount); err != nil {
		return err
	}
	return err
}

func addStakers(gasLimit uint64, st *state.StateDB, master pos.MasterSmartContract, stakers []pos.GenesisStakeInfo, sender common.Address) error {
	var (
		masterAbi abi.ABI
		err error
		input []byte
	)
	vm := newGenesisVM(sender, gasLimit, st)
	if masterAbi, err = abi.JSON(strings.NewReader(master.ABI)); err != nil {
		return err
	}
	for _, staker := range stakers {
		if input, err = masterAbi.Pack("addStaker", staker.Address); err != nil {
			return err
		}
		if _, err = kvm.InternalCall(vm, master.Address, input, big.NewInt(0)); err != nil {
			return err
		}
	}
	return nil
}

func createGenesisNodes(gasLimit uint64, st *state.StateDB, nodes pos.Nodes, masterAddress common.Address) error {
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
		vm := newGenesisVM(n.Owner, gasLimit, st)
		if _, _, _, err = kvm.InternalCreate(vm, n.Address, newCode, big.NewInt(0)); err != nil {
			return err
		}
	}
	return nil
}

func createGenesisStakers(gasLimit uint64, st *state.StateDB, stakers pos.Stakers, masterAddress common.Address, minimumStakes *big.Int) error {
	var (
		err error
		stakerAbi abi.ABI
		input []byte
	)
	if stakerAbi, err = abi.JSON(strings.NewReader(stakers.ABI)); err != nil {
		return err
	}
	for _, staker := range stakers.GenesisInfo {
		if input, err = stakerAbi.Pack("", masterAddress, staker.Owner, big.NewInt(int64(staker.LockedPeriod)), minimumStakes); err != nil {
			return err
		}
		newStakerCode := append(stakers.ByteCode, input...)
		vm := newGenesisVM(staker.Owner, gasLimit, st)
		if _, _, _, err = kvm.InternalCreate(vm, staker.Address, newStakerCode, big.NewInt(0)); err != nil {
			return err
		}
		// stake to staker
		if input, err = stakerAbi.Pack("stake", staker.StakedNode); err != nil {
			return err
		}
		if _, err = kvm.InternalCall(vm, staker.Address, input, staker.StakeAmount); err != nil {
			return err
		}
	}
	return nil
}

func CollectValidators(gasLimit uint64, st *state.StateDB, master pos.MasterSmartContract, sender common.Address) error {
	method := "collectValidators"
	var (
		masterAbi abi.ABI
		err error
		input []byte
	)
	vm := newGenesisVM(sender, gasLimit, st)
	if masterAbi, err = abi.JSON(strings.NewReader(master.ABI)); err != nil {
		return err
	}
	if input, err = masterAbi.Pack(method); err != nil {
		return err
	}
	_, err = kvm.InternalCall(vm, master.Address, input, big.NewInt(0))
	return err
}

// CollectValidatorSet collects new validators list based on current available nodes and start new consensus period
func CollectValidatorSet(bc base.BaseBlockChain, info pos.ConsensusInfo) (*types.ValidatorSet, error) {
	var (
		err error
		n nodeInfo
		input, output []byte
		masterAbi, nodeAbi abi.ABI
		length, startBlock, endBlock uint64
		pubKey *ecdsa.PublicKey
	)

	st, err := bc.State()
	if err != nil {
		return nil, err
	}
	sender := info.Nodes.GenesisInfo[0].Owner
	ctx := kvm.NewInternalKVMContext(sender, bc.CurrentHeader(), bc)
	vm := kvm.NewKVM(ctx, st, kvm.Config{})

	if masterAbi, err = abi.JSON(strings.NewReader(info.Master.ABI)); err != nil {
		return nil, err
	}
	if nodeAbi, err = abi.JSON(strings.NewReader(info.Nodes.ABI)); err != nil {
		return nil, err
	}
	if length, startBlock, endBlock, err = getLatestValidatorsInfo(vm, masterAbi, info.Master.Address, sender); err != nil {
		return nil, err
	}
	validators := make([]*types.Validator, 0)
	for i:=uint64(1); i <= length; i++ {
		var val validator
		if input, err = masterAbi.Pack("GetLatestValidator", i); err != nil {
			return nil, err
		}
		if output, err = kvm.StaticCall(vm, info.Master.Address, input); err != nil {
			return nil, err
		}
		if err = masterAbi.Unpack(&val, "GetLatestValidator", output); err != nil {
			return nil, err
		}
		stakes := calculateVotingPower(val.Stakes)
		if stakes < 0 {
			return nil, fmt.Errorf("invalid stakes")
		}
		// get node info from node address
		if input, err = nodeAbi.Pack("getNodeInfo"); err != nil {
			return nil, err
		}
		if output, err = kvm.StaticCall(vm, val.Node, input); err != nil {
			return nil, err
		}
		if err = nodeAbi.Unpack(&n, "getNodeInfo", output); err != nil {
			return nil, err
		}
		if pubKey, err = crypto.StringToPublicKey(n.NodeId); err != nil {
			return nil, err
		}
		validators = append(validators, types.NewValidator(*pubKey, stakes))
	}
	return types.NewValidatorSet(validators, int64(startBlock), int64(endBlock)), nil
}

// getLatestValidatorsInfo is used after collect validators process is done, node calls this function to get new validators set
func getLatestValidatorsInfo(vm *kvm.KVM, masterAbi abi.ABI, masterAddress, sender common.Address) (uint64, uint64, uint64, error) {
	method := "getLatestValidatorsInfo"
	var (
		err error
		input, output []byte
		info latestValidatorsInfo
	)
	if input, err = masterAbi.Pack(method); err != nil {
		return 0, 0, 0, err
	}
	if output, err = kvm.StaticCall(vm, masterAddress, input); err != nil {
		return 0, 0, 0, err
	}
	if err = masterAbi.Unpack(&info, method, output); err != nil {
		return 0, 0, 0, err
	}
	return info.TotalNodes, info.StartAtBlock, info.EndAtBlock, nil
}

// calculateVotingPower converts stake amount into smaller number that is in int64's scope.
func calculateVotingPower(amount *big.Int) int64 {
	return amount.Div(amount, KAI).Int64()
}
