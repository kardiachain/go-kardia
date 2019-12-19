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
	"crypto/ecdsa"
	"fmt"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
	"math"
	"math/big"
	"strings"
)

// ClaimReward is used to create claimReward transaction
func ClaimReward(height uint64, bc base.BaseBlockChain, state *state.StateDB, txPool *tx_pool.TxPool) (*types.Transaction, error) {
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
	// get node from sender
	if input, err = masterAbi.Pack(methodGetNodeAddressFromOwner, sender); err != nil {
		return nil, err
	}
	if output, err = StaticCall(vm, masterSmartContract.Address, input); err != nil {
		log.Error("fail to get node from sender", "err", err)
		return nil, err
	}
	if err = masterAbi.Unpack(&nodeAddress, methodGetNodeAddressFromOwner, output); err != nil {
		log.Error("fail to unpack output to nodeAddress", "err", err, "output", common.Bytes2Hex(output))
		return nil, err
	}
	// create claimReward transaction
	if input, err = posAbi.Pack(methodClaimReward, nodeAddress.Node, height); err != nil {
		return nil, err
	}
	return generateTransaction(txPool.Nonce(sender), input, &privateKey)
}

// NewConsensusPeriod is created by proposer.
func NewConsensusPeriod(height uint64, bc base.BaseBlockChain, state *state.StateDB, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	var (
		input, output []byte
		posAbi, masterAbi abi.ABI
		err error
		vals validatorsInfo
	)
	sender := bc.Config().BaseAccount.Address
	privateKey := bc.Config().BaseAccount.PrivateKey
	vm := newInternalKVM(sender, bc, state)

	if masterAbi, err = abi.JSON(strings.NewReader(bc.GetConsensusMasterSmartContract().ABI)); err != nil {
		return nil, err
	}
	if input, err = masterAbi.Pack(methodGetLatestValidatorsInfo); err != nil {
		return nil, err
	}
	if output, err = StaticCall(vm, bc.GetConsensusMasterSmartContract().Address, input); err != nil {
		return nil, err
	}
	if err = masterAbi.Unpack(&vals, methodGetLatestValidatorsInfo, output); err != nil {
		return nil, err
	}
	// height must behind EndAtBlock bc.GetFetchNewValidators() blocks.
	if vals.EndAtBlock != height+bc.GetFetchNewValidatorsTime() {
		return nil, nil
	}
	if posAbi, err = abi.JSON(strings.NewReader(PosHandlerAbi)); err != nil {
		return nil, err
	}
	if input, err = posAbi.Pack(methodNewConsensusPeriod, height); err != nil {
		return nil, err
	}
	return generateTransaction(txPool.Nonce(sender), input, &privateKey)
}

// CollectValidatorSet collects new validators list based on current available nodes and start new consensus period
func CollectValidatorSet(bc base.BaseBlockChain) (*types.ValidatorSet, error) {
	var (
		err error
		n nodeInfo
		input, output []byte
		masterAbi, nodeAbi abi.ABI
		length, startBlock, endBlock uint64
		pubKey *ecdsa.PublicKey
	)
	masterAddress := bc.GetConsensusMasterSmartContract().Address
	st, err := bc.State()
	if err != nil {
		return nil, err
	}
	sender := bc.Config().BaseAccount.Address
	ctx := NewInternalKVMContext(sender, bc.CurrentHeader(), bc)
	vm := NewKVM(ctx, st, Config{})

	if masterAbi, err = abi.JSON(strings.NewReader(bc.GetConsensusMasterSmartContract().ABI)); err != nil {
		return nil, err
	}
	if nodeAbi, err = abi.JSON(strings.NewReader(bc.GetConsensusNodeAbi())); err != nil {
		return nil, err
	}
	if length, startBlock, endBlock, err = getLatestValidatorsInfo(vm, masterAbi, masterAddress); err != nil {
		return nil, err
	}
	validators := make([]*types.Validator, 0)
	for i:=uint64(1); i <= length; i++ {
		var val validator
		if input, err = masterAbi.Pack(methodGetLatestValidatorByIndex, i); err != nil {
			return nil, err
		}
		if output, err = StaticCall(vm, masterAddress, input); err != nil {
			return nil, err
		}
		if err = masterAbi.Unpack(&val, methodGetLatestValidatorByIndex, output); err != nil {
			return nil, err
		}
		stakes := calculateVotingPower(val.Stakes)
		if stakes < 0 {
			return nil, fmt.Errorf("invalid stakes")
		}
		// get node info from node address
		if input, err = nodeAbi.Pack(methodGetNodeInfo); err != nil {
			return nil, err
		}
		if output, err = StaticCall(vm, val.Node, input); err != nil {
			return nil, err
		}
		if err = nodeAbi.Unpack(&n, methodGetNodeInfo, output); err != nil {
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
func getLatestValidatorsInfo(vm *KVM, masterAbi abi.ABI, masterAddress common.Address) (uint64, uint64, uint64, error) {
	method := "getLatestValidatorsInfo"
	var (
		err error
		input, output []byte
		info latestValidatorsInfo
	)
	if input, err = masterAbi.Pack(method); err != nil {
		return 0, 0, 0, err
	}
	if output, err = StaticCall(vm, masterAddress, input); err != nil {
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

func generateTransaction(nonce uint64, input []byte, privateKey *ecdsa.PrivateKey) (*types.Transaction, error) {
	return types.SignTx(types.HomesteadSigner{}, types.NewTransaction(
		nonce,
		posHandlerAddress,
		big.NewInt(0),
		calculateGas(input),
		big.NewInt(0),
		input,
	), privateKey)
}

// calculateGas calculates intrinsic gas used for every byte in input data
func calculateGas(data []byte) uint64 {
	gas := TxGas
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/TxDataNonZeroGas < nz {
			return 0
		}
		gas += nz * TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/TxDataZeroGas < z {
			return 0
		}
		gas += z * TxDataZeroGas
	}
	return gas
}

// addLog is used to add rewarded address during claimReward process
func addLog(vm *KVM, rewardedAddress common.Address, rewardedAmount *big.Int, blockHeight uint64) {
	vm.StateDB.AddLog(&types.Log{
		Address: posHandlerAddress,
		Topics:  []common.Hash{common.HexToHash(methodClaimReward), rewardedAddress.Hash()},
		Data:    rewardedAmount.Bytes(),
		BlockHeight: blockHeight,
	})
}

func rewardToNode(nodeAddress common.Address, blockHeight uint64, nodeReward *big.Int, ctx Context, state base.StateDB) error {
	var (
		masterABI abi.ABI
		err error
		input, output []byte
		isRewarded bool
	)
	masterAddress := ctx.Chain.GetConsensusMasterSmartContract().Address
	vm := newInternalKVM(posHandlerAddress, ctx.Chain, state)
	if masterABI, err = abi.JSON(strings.NewReader(ctx.Chain.GetConsensusMasterSmartContract().ABI)); err != nil {
		return err
	}
	// check if node has been rewarded in this blockHeight or not
	if input, err = masterABI.Pack(methodIsRewarded, nodeAddress, blockHeight); err != nil {
		return err
	}
	if output, err = StaticCall(vm, masterAddress, input); err != nil {
		return err
	}
	if err = masterABI.Unpack(&isRewarded, methodIsRewarded, output); err != nil {
		return err
	}
	if isRewarded {
		return fmt.Errorf(fmt.Sprintf("node:%v has been rewarded at block:%v", nodeAddress, blockHeight))
	}
	if input, err = masterABI.Pack(methodSetRewarded, nodeAddress, blockHeight); err != nil {
		return err
	}
	if _, err = InternalCall(vm, masterAddress, input, big.NewInt(0)); err != nil {
		return err
	}
	// update nodeAddress balance
	ctx.Transfer(state, masterAddress, nodeAddress, nodeReward)
	addLog(vm, nodeAddress, nodeReward, blockHeight)
	return nil
}

func rewardToStakers(nodeAddress common.Address, totalStakes *big.Int, stakers map[common.Address]*big.Int, totalReward *big.Int, blockHeight uint64, ctx Context, state base.StateDB) error {
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
		// formula: totalReward*stakedAmount/totalStake
		reward := big.NewInt(0).Div(v, totalStakes)
		reward = big.NewInt(0).Mul(totalReward, reward)
		// call `saveReward` to k to mark reward has been paid
		if input, err = stakerAbi.Pack(methodSaveReward, nodeAddress, blockHeight, reward); err != nil {
			return err
		}
		if _, err = InternalCall(vm, k, input, big.NewInt(0)); err != nil {
			return err
		}
		ctx.Transfer(state, ctx.Chain.GetConsensusMasterSmartContract().Address, k, reward)
		addLog(vm, k, reward, blockHeight)
	}
	return nil
}

func getAvailableNodeInfo(bc base.BaseBlockChain, st base.StateDB, sender, node common.Address) (common.Address, *big.Int, map[common.Address]*big.Int, error) {
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
	if input, err = masterAbi.Pack(methodGetAvailableNodeIndex, node); err != nil {
		return owner, stakes, stakers, err
	}
	if output, err = StaticCall(vm, master.Address, input); err != nil {
		return owner, stakes, stakers, err
	}
	if err = masterAbi.Unpack(&index, methodGetAvailableNodeIndex, output); err != nil {
		return owner, stakes, stakers, err
	}
	if index.Uint64() == 0 {
		return owner, stakes, stakers, fmt.Errorf(fmt.Sprintf("cannot find node:%v info", node.Hex()))
	}
	if input, err = masterAbi.Pack(methodGetAvailableNode, index); err != nil {
		return owner, stakes, stakers, err
	}
	if output, err = StaticCall(vm, master.Address, input); err != nil {
		return owner, stakes, stakers, err
	}
	if err = masterAbi.Unpack(&nodeInfo, methodGetAvailableNode, output); err != nil {
		return owner, stakes, stakers, err
	}
	for i := uint64(1); i < nodeInfo.TotalStaker; i++ {
		var info stakerInfo
		if input, err = masterAbi.Pack(methodGetStakerInfo, node, i); err != nil {
			return owner, stakes, stakers, err
		}
		if output, err = StaticCall(vm, master.Address, input); err != nil {
			return owner, stakes, stakers, err
		}
		if err = masterAbi.Unpack(&info, methodGetStakerInfo, output); err != nil {
			return owner, stakes, stakers, err
		}
		stakers[info.Staker] = info.Amount
	}
	return nodeInfo.Owner, nodeInfo.Stakes, stakers, err
}

func getNodeInfo(bc base.BaseBlockChain, st base.StateDB, sender, node common.Address) (*nodeInfo, error) {
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
	if input, err = nodeAbi.Pack(methodGetNodeInfo); err != nil {
		return nil, err
	}
	if output, err = StaticCall(vm, node, input); err != nil {
		return nil, err
	}
	if err = nodeAbi.Unpack(&nInfo, methodGetNodeInfo, output); err != nil {
		return nil, err
	}
	return &nInfo, nil
}
