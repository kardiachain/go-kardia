package permissioned

import (
	"math/big"
	"strings"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/kai/base"
)

const (
	mockSmartContractCallSenderAccount = "0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5"
	KardiaPermissionSmcIndex = 4
)


// PermissionSmcUtil wraps all utility methods related to permission smc
type PermissionSmcUtil struct {
	Abi             *abi.ABI
	StateDb         *state.StateDB
	ContractAddress *common.Address
	SenderAddress   *common.Address
	bc              *base.BaseBlockChain
}

func NewSmcPermissionUtil(bc base.BaseBlockChain) (*PermissionSmcUtil, error) {
	stateDb, err := bc.State()
	if err != nil {
		log.Error("Error get state", "err", err)
		return nil, err
	}
	permissionSmcAddr, permissionSmcAbi := configs.GetContractDetailsByIndex(KardiaPermissionSmcIndex)
	if permissionSmcAbi == "" {
		log.Error("Error getting abi by index")
		return nil, err
	}
	abi, err := abi.JSON(strings.NewReader(permissionSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	senderAddr := common.HexToAddress(mockSmartContractCallSenderAccount)
	return &PermissionSmcUtil{Abi: &abi, StateDb: stateDb, ContractAddress: &permissionSmcAddr, SenderAddress: &senderAddr,
		bc: &bc}, nil
}

// IsValidNode executes smart contract to check if a node with specified pubkey and nodeType is valid
func (s *PermissionSmcUtil) IsValidNode(pubkey string, nodeType int64) (bool, error) {
	checkNodeValidInput, err := s.Abi.Pack("isValidNode", pubkey, big.NewInt(nodeType))
	if err != nil {
		log.Error("Error packing check valid node input", "err", err)
		return false, err
	}
	checkNodeValidResult, err := kardia.CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress, *s.bc,
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
	getNodeInfoResult, err := kardia.CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress,
		*s.bc, getNodeInfoInput, s.StateDb)
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
	err = s.Abi.Unpack(&nodeInfo, "getNodeInfo", getNodeInfoResult)
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
	checkValidatorResult, err := kardia.CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress,
		*s.bc, checkValidatorInput, s.StateDb)
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
	votingPower *big.Int, listenAddr string, state *state.ManagedState) (*types.Transaction, error) {
	addNodeInput, err := s.Abi.Pack("addNode", pubkey, address, big.NewInt(nodeType), votingPower, listenAddr)
	if err != nil {
		log.Error("Error packing add node input", "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(kardia.GetPrivateKeyToCallKardiaSmc(), *s.ContractAddress, addNodeInput,
		state), nil
}

// RemoveNodeForPrivateChain returns tx to remove a node with specified pubkey and nodeType from a private chain
// Only admins can call this function
func (s *PermissionSmcUtil) RemoveNodeForPrivateChain(pubkey string, stateDb *state.ManagedState) (*types.Transaction, error) {
	removeNodeInput, err := s.Abi.Pack("removeNode", pubkey)
	if err != nil {
		log.Error("Error packing remove node input", "err", err)
		return nil, err
	}
	return tool.GenerateSmcCall(kardia.GetPrivateKeyToCallKardiaSmc(), *s.ContractAddress, removeNodeInput, stateDb), nil
}

// GetAdminNodeByIndex executes smart contract to get info of an initial node, including public key, address, listen address,
// voting power, node type
func (s *PermissionSmcUtil) GetAdminNodeByIndex(index int64) (string, common.Address, string, *big.Int, *big.Int, error) {
	getInitialNodeInput, err := s.Abi.Pack("getInitialNodeByIndex", big.NewInt(index))
	if err != nil {
		log.Error("Error packing initial node input", "err", err)
		return "", common.Address{}, "", nil, nil, err
	}
	getInitialNodeResult, err := kardia.CallStaticKardiaMasterSmc(*s.SenderAddress, *s.ContractAddress,
		*s.bc, getInitialNodeInput, s.StateDb)
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
	err = s.Abi.Unpack(&initialNodeInfo, "getInitialNodeByIndex", getInitialNodeResult)
	if err != nil {
		log.Error("Error calling permission contract", "err", err)
		return "", common.Address{}, "", nil, nil, err
	}
	return initialNodeInfo.Publickey, initialNodeInfo.Addr, initialNodeInfo.ListenAddr, initialNodeInfo.VotingPower, initialNodeInfo.NodeType, nil
}
