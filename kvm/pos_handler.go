package kvm

import (
	"fmt"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
	"math/big"
	"strings"
)

const (
	PosHandlerAbi = `[
	{
		"constant": false,
		"inputs": [],
		"name": "newConsensusPeriod",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "node",
				"type": "address"
			},
			{
				"name": "blockHeight",
				"type": "uint64"
			}
		],
		"name": "claimReward",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
)

var posHandlerAddress = common.BytesToAddress([]byte{5})

type (
	availableNode struct {
		NodeAddress common.Address `abi:"nodeAddress"`
		Owner common.Address `abi:"owner"`
		Stakes *big.Int `abi:"stakes"`
		TotalStaker uint64 `abi:"totalStaker"`
	}
	stakerInfo struct {
		Staker common.Address `abi:"staker"`
		Amount *big.Int `abi:"amount"`
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
	node struct {
		Node common.Address `abi:"node"`
		BlockHeight uint64  `abi:"blockHeight"`
	}
	// posHandler.
	 posHandler struct {}
)

func (p *posHandler) RequiredGas(input []byte) uint64 {
	return 0
}

func (p *posHandler) Run(in []byte, contract *Contract, ctx Context, state base.StateDB) ([]byte, error) {
	pAbi, err := abi.JSON(strings.NewReader(PosHandlerAbi))
	if err != nil {
		return nil, err
	}
	method, _, err := abi.GenerateInputStruct(pAbi, in)
	if err != nil {
		return nil, err
	}
	switch method.Name {
	case "claimReward":
		return in, handleClaimReward(method, in, contract, ctx, state)
	}
	return in, nil
}

func handleClaimReward(method *abi.Method, input []byte, contract *Contract, ctx Context, state base.StateDB) error {
	var (
		owner common.Address
		stakeAmount *big.Int
		stakers map[common.Address]*big.Int
		nInfo *nodeInfo
		err error
	)
	var n node
	if err = method.Inputs.Unpack(&n, input[4:]); err != nil {
		return err
	}
	// validate if node's owner is sender or not.
	if owner, stakeAmount, stakers, err = getAvailableNodeInfo(ctx.Chain, state, contract.caller.Address(), n.Node); err != nil {
		return err
	}
	if owner.Hex() != contract.caller.Address().Hex() {
		return fmt.Errorf(fmt.Sprintf("sender:%v is not owner of node:%v", contract.caller.Address().Hex(), n.Node.Hex()))
	}
	block := ctx.Chain.CurrentBlock()
	if block.Height() <= 1 {
		return fmt.Errorf("block <= 1 cannot join claim reward")
	}
	// get reward from previous block gasUsed + blockReward
	blockReward, _ := big.NewInt(0).SetString(ctx.Chain.GetBlockReward().String(), 10)
	previousHeader := ctx.Chain.GetBlockByHeight(block.Height()-1).Header()
	if previousHeader.GasUsed > 0 {
		blockReward = blockReward.Add(blockReward, big.NewInt(0).SetUint64(previousHeader.GasUsed))
	}
	if nInfo, err = getNodeInfo(ctx.Chain, state, owner, n.Node); err != nil {
		return err
	}
	stakersReward := big.NewInt(0).Mul(blockReward, big.NewInt(int64(nInfo.RewardPercentage)))
	stakersReward = big.NewInt(0).Div(stakersReward, big.NewInt(100))
	nodeReward := blockReward.Sub(blockReward, stakersReward)

	// reward to node
	if err = rewardToNode(n.Node, block.Height()-1, nodeReward, ctx, state); err != nil {
		return err
	}
	// reward to stakers
	return rewardToStakers(n.Node, stakeAmount, stakers, stakersReward, block.Height()-1, ctx, state)
}

func rewardToNode(nodeAddress common.Address, blockHeight uint64, nodeReward *big.Int, ctx Context, state base.StateDB) error {
	var (
		masterABI abi.ABI
		err error
		input []byte
	)
	vm := newInternalKVM(posHandlerAddress, ctx.Chain, state)
	if masterABI, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusMasterSmartContract().ABI)); err != nil {
		return err
	}
	if input, err = masterABI.Pack("setRewarded", nodeAddress, blockHeight); err != nil {
		return err
	}
	if _, err = InternalCall(vm, ctx.Chain.GetConsensusMasterSmartContract().Address, input, big.NewInt(0)); err != nil {
		return err
	}
	ctx.Transfer(state, ctx.Chain.GetConsensusMasterSmartContract().Address, nodeAddress, nodeReward)
	return nil
}

func rewardToStakers(nodeAddress common.Address, totalStakes *big.Int, stakers map[common.Address]*big.Int, totalReward *big.Int, blockHeight uint64, ctx Context, state base.StateDB) error {
	method := "saveReward"
	var (
		err error
		input []byte
		stakerAbi abi.ABI
	)
	vm := newInternalKVM(posHandlerAddress, ctx.Chain, state)
	if stakerAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusStakerAbi())); err != nil {
		return err
	}
	for k, v := range stakers {
		reward := totalReward.Mul(totalReward, v)
		reward = reward.Div(reward, totalStakes)

		// call `saveReward` to k to mark reward has been paid
		if input, err = stakerAbi.Pack(method, nodeAddress, blockHeight, reward); err != nil {
			return err
		}
		if _, err = InternalCall(vm, k, input, big.NewInt(0)); err != nil {
			return err
		}
		ctx.Transfer(state, ctx.Chain.GetConsensusMasterSmartContract().Address, k, reward)
	}
	return nil
}

func getAvailableNodeInfo(bc base.BaseBlockChain, st base.StateDB, sender, node common.Address) (common.Address, *big.Int, map[common.Address]*big.Int, error) {
	methods := []string{"getAvailableNodeIndex", "getAvailableNode", "getStakerInfo"}
	master := bc.GetConsensusMasterSmartContract()
	var (
		err error
		input []byte
		output []byte
		stakes *big.Int
		index *big.Int
		nodeInfo availableNode
		masterAbi abi.ABI
	)
	owner := common.Address{}
	stakers := make(map[common.Address]*big.Int)
	vm := newInternalKVM(sender, bc, st)
	if masterAbi, err = abi.JSON(strings.NewReader(master.ABI)); err != nil {
		return owner, stakes, stakers, err
	}
	// get nodeIndex
	if input, err = masterAbi.Pack(methods[0], node); err != nil {
		return owner, stakes, stakers, err
	}
	if output, err = StaticCall(vm, master.Address, input); err != nil {
		return owner, stakes, stakers, err
	}
	if err = masterAbi.Unpack(&index, methods[0], output); err != nil {
		return owner, stakes, stakers, err
	}
	if index.Uint64() == 0 {
		return owner, stakes, stakers, fmt.Errorf(fmt.Sprintf("cannot find node:%v info", node.Hex()))
	}
	if input, err = masterAbi.Pack(methods[1], index); err != nil {
		return owner, stakes, stakers, err
	}
	if output, err = StaticCall(vm, master.Address, input); err != nil {
		return owner, stakes, stakers, err
	}
	if err = masterAbi.Unpack(&nodeInfo, methods[1], output); err != nil {
		return owner, stakes, stakers, err
	}
	for i := uint64(1); i < nodeInfo.TotalStaker; i++ {
		var info stakerInfo
		if input, err = masterAbi.Pack(methods[2], node, i); err != nil {
			return owner, stakes, stakers, err
		}
		if output, err = StaticCall(vm, master.Address, input); err != nil {
			return owner, stakes, stakers, err
		}
		if err = masterAbi.Unpack(&info, methods[2], output); err != nil {
			return owner, stakes, stakers, err
		}
		stakers[info.Staker] = info.Amount
	}
	return nodeInfo.Owner, nodeInfo.Stakes, stakers, err
}

func getNodeInfo(bc base.BaseBlockChain, st base.StateDB, sender, node common.Address) (*nodeInfo, error) {
	method := "getNodeInfo"
	var (
		input, output []byte
		nodeAbi abi.ABI
		nInfo nodeInfo
		err error
	)
	vm := newInternalKVM(sender, bc, st)
	if nodeAbi, err = abi.JSON(strings.NewReader(bc.GetConsensusNodeAbi())); err != nil {
		return nil, err
	}
	if input, err = nodeAbi.Pack(method); err != nil {
		return nil, err
	}
	if output, err = StaticCall(vm, node, input); err != nil {
		return nil, err
	}
	if err = nodeAbi.Unpack(&nInfo, method, output); err != nil {
		return nil, err
	}
	return &nInfo, nil
}

func ClaimReward(bc base.BaseBlockChain, state *state.StateDB, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	methods := []string{"getNodeAddressFromOwner", "claimReward"}
	type nodeAddressFromOwner struct {
		Node common.Address `abi:"node"`
	}
	var (
		posAbi, masterAbi abi.ABI
		err error
		input, output []byte
		nodeAddress nodeAddressFromOwner
	)
	sender := bc.Config().BaseAccount.Address
	privateKey := bc.Config().BaseAccount.PrivateKey
	vm := newInternalKVM(sender, bc, state)

	if posAbi, err = abi.JSON(strings.NewReader(PosHandlerAbi)); err != nil {
		log.Error("fail to init posAbi", "err", err)
		return nil, err
	}
	masterSmartContract := bc.GetConsensusMasterSmartContract()
	if masterAbi, err = abi.JSON(strings.NewReader(masterSmartContract.ABI)); err != nil {
		log.Error("fail to init masterAbi", "err", err)
		return nil, err
	}

	// test isValidator
	if input, err = masterAbi.Pack("IsValidator", sender); err != nil{
		return nil, err
	}
	if output, err = StaticCall(vm, masterSmartContract.Address, input); err != nil {
		return nil, err
	}
	var result bool
	if err = masterAbi.Unpack(&result, "IsValidator", output); err != nil {
		return nil, err
	}
	if !result {
		return nil, fmt.Errorf("user is not validator")
	}

	// get node from sender
	if input, err = masterAbi.Pack(methods[0], sender); err != nil {
		return nil, err
	}
	if output, err = StaticCall(vm, masterSmartContract.Address, input); err != nil {
		log.Error("fail to get node from sender", "err", err)
		return nil, err
	}
	if err = masterAbi.Unpack(&nodeAddress, methods[0], output); err != nil {
		log.Error("fail to unpack output to nodeAddress", "err", err, "output", common.Bytes2Hex(output))
		return nil, err
	}
	// create claimReward transaction
	if input, err = posAbi.Pack("claimReward", nodeAddress.Node, bc.CurrentHeader().Height); err != nil {
		return nil, err
	}
	gas, err := EstimateGas(vm, posHandlerAddress, input)
	if err != nil {
		return nil, err
	}
	return types.SignTx(types.NewTransaction(
		txPool.GetAddressState(sender),
		posHandlerAddress,
		big.NewInt(0),
		gas,
		big.NewInt(0),
		input,
	), &privateKey)
}