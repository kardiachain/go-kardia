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

package permissioned

import (
	"crypto/ecdsa"
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	kvm2 "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/types"
)

var MaximumGasToCallStaticFunction = uint(4000000)

// PermissionSmcUtil wraps all utility methods related to permission smc
type PermissionSmcUtil struct {
	Abi              *abi.ABI
	StateDb          *state.StateDB
	ContractAddress  *common.Address
	SenderAddress    *common.Address
	SenderPrivateKey *ecdsa.PrivateKey
	bc               base.BaseBlockChain
}

func NewSmcPermissionUtil(bc base.BaseBlockChain) (*PermissionSmcUtil, error) {
	stateDb, err := bc.State()
	if err != nil {
		log.Error("Error get state", "err", err)
		return nil, err
	}
	permissionSmcAddr := common.HexToAddress(configs.KardiaPermissionSmcAddress)
	permissionSmcAbi := configs.GetContractABIByAddress(configs.KardiaPermissionSmcAddress)
	if permissionSmcAbi == "" {
		log.Error("Error getting abi by index")
		return nil, err
	}
	abi, err := abi.JSON(strings.NewReader(permissionSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	return &PermissionSmcUtil{Abi: &abi, StateDb: stateDb, ContractAddress: &permissionSmcAddr,
		SenderAddress: bc.P2P().Address(), bc: bc, SenderPrivateKey: bc.P2P().PrivKey()}, nil
}

// IsValidNode executes smart contract to check if a node with specified pubkey and nodeType is valid
func (s *PermissionSmcUtil) IsValidNode(pubkey string, nodeType int64) (bool, error) {
	checkNodeValidInput, err := s.Abi.Pack("isValidNode", pubkey, big.NewInt(nodeType))
	if err != nil {
		log.Error("Error packing check valid node input", "err", err)
		return false, err
	}
	checkNodeValidResult, err := CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress, s.bc,
		checkNodeValidInput, s.StateDb)
	if err != nil {
		log.Error("Error call permission contract", "err", err)
		return false, err
	}
	result := big.NewInt(0).SetBytes(checkNodeValidResult)
	return result.Cmp(big.NewInt(1)) == 0, nil
}

// GetNodeInfo executes smart contract to get info of a node specified by pubkey, returns address, nodeType, votingPower and listenAddress
func (s *PermissionSmcUtil) GetNodeInfo(pubkey string) (common.Address, *big.Int, *big.Int, string, error) {
	getNodeInfoInput, err := s.Abi.Pack("getNodeInfo", pubkey)
	if err != nil {
		log.Error("Error packing get node info input", "err", err)
		return common.Address{}, nil, nil, "", err
	}
	getNodeInfoResult, err := CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress,
		s.bc, getNodeInfoInput, s.StateDb)
	if err != nil {
		log.Error("Error call permission contract", "err", err)
		return common.Address{}, nil, nil, "", err
	}
	var nodeInfo struct {
		Addr          common.Address
		VotingPower   *big.Int
		NodeType      *big.Int
		ListenAddress string
	}
	err = s.Abi.UnpackIntoInterface(&nodeInfo, "getNodeInfo", getNodeInfoResult)
	if err != nil {
		log.Error("Error unpacking node info", "err", err)
		return common.Address{}, nil, nil, "", err
	}
	return nodeInfo.Addr, nodeInfo.NodeType, nodeInfo.VotingPower, nodeInfo.ListenAddress, nil
}

// IsValidator executes smart contract to check if a node with specified pubkey is validator
func (s *PermissionSmcUtil) IsValidator(pubkey string) (bool, error) {
	checkValidatorInput, err := s.Abi.Pack("isValidator", pubkey)
	if err != nil {
		log.Error("Error packing check validator input", "err", err)
		return false, err
	}
	checkValidatorResult, err := CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress,
		s.bc, checkValidatorInput, s.StateDb)
	if err != nil {
		log.Error("Error call permission contract", "err", err)
		return false, err
	}
	result := big.NewInt(0).SetBytes(checkValidatorResult)
	return result.Cmp(big.NewInt(1)) == 0, nil
}

// AddNodeForPrivateChain returns tx to add a node with specified pubkey, nodeType, address and votingPower to list of nodes
// of a private chain. If votingPower > 0, added node is validator. Only admins can call this function
func (s *PermissionSmcUtil) AddNodeForPrivateChain(pubkey string, nodeType int64, address common.Address,
	votingPower *big.Int, listenAddr string, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	addNodeInput, err := s.Abi.Pack("addNode", pubkey, address, big.NewInt(nodeType), votingPower, listenAddr)
	if err != nil {
		log.Error("Error packing add node input", "err", err)
		return nil, err
	}
	return tx_pool.GenerateSmcCall(s.SenderPrivateKey, *s.ContractAddress, addNodeInput,
		txPool, false), nil
}

// RemoveNodeForPrivateChain returns tx to remove a node with specified pubkey and nodeType from a private chain
// Only admins can call this function
func (s *PermissionSmcUtil) RemoveNodeForPrivateChain(pubkey string, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	removeNodeInput, err := s.Abi.Pack("removeNode", pubkey)
	if err != nil {
		log.Error("Error packing remove node input", "err", err)
		return nil, err
	}
	return tx_pool.GenerateSmcCall(s.SenderPrivateKey, *s.ContractAddress, removeNodeInput, txPool, false), nil
}

// GetAdminNodeByIndex executes smart contract to get info of an initial node, including public key, address, listen address,
// voting power, node type
func (s *PermissionSmcUtil) GetAdminNodeByIndex(index int64) (string, common.Address, string, *big.Int, *big.Int, error) {
	getInitialNodeInput, err := s.Abi.Pack("getInitialNodeByIndex", big.NewInt(index))
	if err != nil {
		log.Error("Error packing initial node input", "err", err)
		return "", common.Address{}, "", nil, nil, err
	}
	getInitialNodeResult, err := CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress,
		s.bc, getInitialNodeInput, s.StateDb)
	if err != nil {
		log.Error("Error calling permission contract", "err", err)
		return "", common.Address{}, "", nil, nil, err
	}
	var initialNodeInfo struct {
		Publickey   string
		Addr        common.Address
		ListenAddr  string
		VotingPower *big.Int
		NodeType    *big.Int
	}
	err = s.Abi.UnpackIntoInterface(&initialNodeInfo, "getInitialNodeByIndex", getInitialNodeResult)
	if err != nil {
		log.Error("Error calling permission contract", "err", err)
		return "", common.Address{}, "", nil, nil, err
	}
	return initialNodeInfo.Publickey, initialNodeInfo.Addr, initialNodeInfo.ListenAddr, initialNodeInfo.VotingPower, initialNodeInfo.NodeType, nil
}

// The following function is just call the master smc and return result in bytes format
func CallStaticKardiaMasterSmc(from common.Address, to common.Address, bc base.BaseBlockChain, input []byte, statedb *state.StateDB) (result []byte, err error) {
	context := kvm2.NewKVMContextFromDualNodeCall(from, bc.CurrentHeader(), bc)
	vmenv := kvm.NewKVM(context, statedb, kvm.Config{})
	sender := kvm.AccountRef(from)
	ret, _, err := vmenv.StaticCall(sender, to, input, uint64(MaximumGasToCallStaticFunction))
	if err != nil {
		return make([]byte, 0), err
	}
	return ret, nil
}
