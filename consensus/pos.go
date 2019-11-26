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

package consensus

import (
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/pos"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"math/big"
	"strings"
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
	if err = createMaster(gasLimit, st, consensusInfo.Master, consensusInfo.MaxValidators, consensusInfo.ConsensusPeriodInBlock, sender); err != nil {
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
