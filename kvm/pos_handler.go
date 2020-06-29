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

package kvm

import (
	"fmt"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"math/big"
	"strings"
)

const (
	PosHandlerAbi = `[
	{
		"constant": false,
		"inputs": [],
		"name": "createStaker",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "publicKey",
				"type": "string"
			},
			{
				"name": "nodeName",
				"type": "string"
			},
			{
				"name": "rewardPercentage",
				"type": "uint16"
			},
			{
				"name": "lockedPeriod",
				"type": "uint64"
			},
			{
				"name": "minimumStakes",
				"type": "uint256"
			}
		],
		"name": "createNode",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "blockHeight",
				"type": "uint64"
			}
		],
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
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "node",
				"type": "address"
			},
			{
				"name": "maxViolatePercentage",
				"type": "uint64"
			}
		],
		"name": "isViolatedNode",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
	methodSetRewarded = "setRewarded"
	methodIsRewarded = "isRewarded"
	methodSaveReward = "saveReward"
	methodGetNodeInfo = "getNodeInfo"
	methodGetNodeAddressFromOwner = "getNodeAddressFromOwner"
	methodClaimReward = "claimReward"
	methodGetAvailableNodeIndex = "getAvailableNodeIndex"
	methodGetAvailableNode = "getAvailableNode"
	methodGetStakerInfo = "getStakerInfo"
	methodNewConsensusPeriod = "newConsensusPeriod"
	methodGetLatestValidatorsInfo = "getLatestValidatorsInfo"
	methodGetLatestValidatorByIndex = "getLatestValidatorByIndex"
	methodCollectValidators = "collectValidators"
	methodIsViolatedNode = "isViolatedNode"
	methodGetRejectedValidatedInfo = "getRejectedValidatedInfo"
	methodAddNode = "addNode"
	methodAddStaker = "addStaker"
	methodCreateNode = "createNode"
	methodCreateStaker = "createStaker"
)

var (
	KAI, _ = big.NewInt(0).SetString("100000000000000000", 10) // 10**18
	posHandlerAddress = common.BytesToAddress([]byte{5})
)

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
		RewardPercentage uint16 `abi:"rewardPercentage"`
		Balance *big.Int `abi:"balance"`
	}
	node struct {
		Node common.Address `abi:"node"`
		BlockHeight uint64  `abi:"blockHeight"`
	}
	validatorsInfo struct {
		TotalNodes uint64 `abi:"totalNodes"`
		StartAtBlock uint64 `abi:"startAtBlock"`
		EndAtBlock uint64 `abi:"endAtBlock"`
	}
	nodeAddressFromOwner struct {
		Node common.Address `abi:"node"`
	}
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
	rejectedValidatedInfo struct {
		RejectedBlocks  *big.Int `abi:"rejectedBlocks"`
		ValidatedBlocks *big.Int `abi:"validatedBlocks"`
	}
	createNodeStruct struct {
		PublicKey        string  `abi:"publicKey"`
		NodeName         string  `abi:"nodeName"`
		RewardPercentage string  `abi:"rewardPercentage"`
		LockedPeriod     uint64  `abi:"lockedPeriod"`
		MinimumStakes    *big.Int`abi:"minimumStakes"`
	}
	// posHandler.
	 posHandler struct {}
)

func (p *posHandler) RequiredGas(input []byte) uint64 {
	return 0
}

func (p *posHandler) Run(in []byte, contract *Contract, ctx Context, state base.StateDB) ([]byte, error) {
	var (
		pAbi abi.ABI
		err error
		method *abi.Method
	)
	if pAbi, err = abi.JSON(strings.NewReader(PosHandlerAbi)); err != nil {
		return nil, err
	}
	if method, _, err = abi.GenerateInputStruct(pAbi, in); err != nil {
		return nil, err
	}
	switch method.Name {
	case methodClaimReward:
		return in, handleClaimReward(method, in, contract, ctx, state)
	case methodNewConsensusPeriod:
		return in, handleNewConsensusPeriod(method, in, contract, ctx, state)
	case methodIsViolatedNode:
		return handleIsViolatedNode(method, in, contract, ctx, state)
	case methodCreateNode:
		return in, handleCreateNode(method, in, contract, ctx, state)
	case methodCreateStaker:
		return in, handleCreateStaker(contract, ctx, state)
	}
	return in, nil
}

// handleClaimReward handles claimReward transaction sent from last proposer
func handleClaimReward(method *abi.Method, input []byte, contract *Contract, ctx Context, state base.StateDB) error {
	var (
		owner common.Address
		stakeAmount *big.Int
		stakers map[common.Address]*big.Int
		nInfo *nodeInfo
		err error
		n node
	)
	if err = method.Inputs.Unpack(&n, input[4:]); err != nil {
		return err
	}
	// validate if node's owner is sender or not.
	if owner, stakeAmount, stakers, err = getAvailableNodeInfo(ctx.Chain, state, contract.Caller(), n.Node); err != nil {
		return err
	}
	if !owner.Equal(contract.Caller()) {
		return fmt.Errorf(fmt.Sprintf("sender:%v is not owner of node:%v", contract.Caller().Hex(), n.Node.Hex()))
	}
	block := ctx.Chain.CurrentBlock()
	if block.Height() <= 1 {
		return fmt.Errorf("block <= 1 cannot join claim reward")
	}
	claimedBlock := ctx.Chain.GetBlockByHeight(n.BlockHeight)
	if claimedBlock == nil {
		return fmt.Errorf(fmt.Sprintf("cannot find block:%v", n.BlockHeight))
	}
	// validate if node's owner is
	if !claimedBlock.Header().Validator.Equal(contract.Caller()) {
		return fmt.Errorf(fmt.Sprintf("caller:%v is not block:%v validator, expected:%v", contract.Caller().Hex(), n.BlockHeight, claimedBlock.Header().Validator.Hex()))
	}

	// get reward from previous block gasUsed + blockReward
	blockReward, _ := big.NewInt(0).SetString(ctx.Chain.GetBlockReward().String(), 10)

	if claimedBlock.Header().GasUsed > 0 {
		blockReward = big.NewInt(0).Add(blockReward, big.NewInt(0).SetUint64(claimedBlock.Header().GasUsed))
	}
	if nInfo, err = getNodeInfo(ctx.Chain, state, owner, n.Node); err != nil {
		return err
	}
	stakersReward := big.NewInt(0).Mul(blockReward, big.NewInt(int64(nInfo.RewardPercentage)))
	stakersReward = big.NewInt(0).Div(stakersReward, big.NewInt(100))
	nodeReward := big.NewInt(0).Sub(blockReward, stakersReward)
	// reward to node
	if err = rewardToNode(n.Node, n.BlockHeight, nodeReward, ctx, state); err != nil {
		return err
	}
	// reward to stakers
	return rewardToStakers(n.Node, stakeAmount, stakers, stakersReward, n.BlockHeight, ctx, state)
}

func handleNewConsensusPeriod(method *abi.Method, input []byte, contract *Contract, ctx Context, state base.StateDB) error {
	type inputStruct struct {
		BlockHeight uint64 `abi:"blockHeight"`
	}
	var (
		masterAbi abi.ABI
		err error
		is inputStruct
	)
	if err = method.Inputs.Unpack(&is, input[4:]); err != nil {
		return err
	}
	sender := contract.CallerAddress
	block := ctx.Chain.GetBlockByHeight(is.BlockHeight)
	header := block.Header()
	vm := newInternalKVM(sender, ctx.Chain, state)
	if masterAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusMasterSmartContract().ABI)); err != nil {
		return err
	}
	// compare node address with block's proposer
	if header.Validator.Hex() != sender.Hex() {
		return fmt.Errorf(fmt.Sprintf("sender:%v is not proposer of block %v - block's proposer is %v", sender.Hex(), is.BlockHeight, header.Validator.Hex()))
	}
	// if matched, call method collectValidators
	if input, err = masterAbi.Pack(methodCollectValidators); err != nil {
		return err
	}
	if _, err = InternalCall(vm, ctx.Chain.GetConsensusMasterSmartContract().Address, input, big.NewInt(0)); err != nil {
		log.Error("collect validators failed", "err", err)
		return err
	}
	return nil
}

func handleIsViolatedNode(method *abi.Method, input []byte, contract *Contract, ctx Context, state base.StateDB) ([]byte, error) {
	// validate if request has been sent from master or not.
	if !contract.caller.Address().Equal(ctx.Chain.GetConsensusMasterSmartContract().Address) {
		return nil, fmt.Errorf("only master smart contract has permission to access this function")
	}

	type inputStruct struct {
		Node                 common.Address `abi:"node"`
		MaxViolatePercentage uint64         `abi:"maxViolatePercentage"`
	}
	var (
		nodeAbi abi.ABI
		err error
		is inputStruct
		rejectedValidatedInfo rejectedValidatedInfo
		output []byte
	)
	result := make([]byte, 32)
	vm := newInternalKVM(contract.CallerAddress, ctx.Chain, state)
	if nodeAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusNodeAbi())); err != nil {
		return result, err
	}
	if err = method.Inputs.Unpack(&is, input[4:]); err != nil {
		return result, err
	}
	if input, err = nodeAbi.Pack(methodGetRejectedValidatedInfo); err != nil {
		return result, err
	}
	if output, err = StaticCall(vm, is.Node, input); err != nil {
		return result, err
	}
	if err = nodeAbi.Unpack(&rejectedValidatedInfo, methodGetRejectedValidatedInfo, output); err != nil {
		return result, err
	}

	// calculate rejected/(rejected+validated) >= maxViolate or not.
	rejected := big.NewFloat(0).SetUint64(rejectedValidatedInfo.RejectedBlocks.Uint64())
	validated := big.NewFloat(0).SetUint64(rejectedValidatedInfo.ValidatedBlocks.Uint64())
	rs, _ := big.NewFloat(0).Quo(rejected, big.NewFloat(0).Add(rejected, validated)).Float64()

	maxViolatePercentage := float64(is.MaxViolatePercentage)/float64(100)
	if rs >= maxViolatePercentage {
		// assign 1 at the end of []byte to mark it as true value
		result[31] = 1
	}
	return result, nil
}

func handleCreateNode(method *abi.Method, input []byte, contract *Contract, ctx Context, state base.StateDB) error {
	var (
		err error
		masterAbi, nodeAbi abi.ABI
		node createNodeStruct
		nodeAddress common.Address
	)
	nodeVm := newInternalKVM(contract.CallerAddress, ctx.Chain, state)
	vm := newInternalKVM(posHandlerAddress, ctx.Chain, state)
	if err = method.Inputs.Unpack(&node, input[4:]); err != nil {
		return err
	}
	if masterAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusMasterSmartContract().ABI)); err != nil {
		return err
	}
	if nodeAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusNodeAbi())); err != nil {
		return err
	}
	// create node
	if input, err = nodeAbi.Pack("", ctx.Chain.GetConsensusMasterSmartContract().Address, node.PublicKey, node.NodeName, node.RewardPercentage, node.LockedPeriod, node.MinimumStakes); err != nil {
		return err
	}
	if _, nodeAddress, _, err = InternalCreate(nodeVm, nil, input, big.NewInt(0)); err != nil {
		return err
	}
	// call addNode on master to save nodeAddress
	if input, err = masterAbi.Pack(methodAddNode, nodeAddress); err != nil {
		return err
	}
	if _, err = InternalCall(vm, ctx.Chain.GetConsensusMasterSmartContract().Address, input, big.NewInt(0)); err != nil {
		return err
	}
	return nil
}

func handleCreateStaker(contract *Contract, ctx Context, state base.StateDB) error {
	var (
		input []byte
		err error
		masterAbi, nodeAbi abi.ABI
		stakerAddress common.Address
	)
	stakerVm := newInternalKVM(contract.CallerAddress, ctx.Chain, state)
	vm := newInternalKVM(posHandlerAddress, ctx.Chain, state)
	if masterAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusMasterSmartContract().ABI)); err != nil {
		return err
	}
	if nodeAbi, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusNodeAbi())); err != nil {
		return err
	}
	// create staker
	if input, err = nodeAbi.Pack("", ctx.Chain.GetConsensusMasterSmartContract().Address); err != nil {
		return err
	}
	if _, stakerAddress, _, err = InternalCreate(stakerVm, nil, input, big.NewInt(0)); err != nil {
		return err
	}
	// call addNode on master to save nodeAddress
	if input, err = masterAbi.Pack(methodAddStaker, stakerAddress); err != nil {
		return err
	}
	if _, err = InternalCall(vm, ctx.Chain.GetConsensusMasterSmartContract().Address, input, big.NewInt(0)); err != nil {
		return err
	}
	return nil
}
