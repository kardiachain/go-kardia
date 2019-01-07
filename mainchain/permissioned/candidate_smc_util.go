package permissioned

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/pkg/errors"
)

type CandidateSmcUtil struct {
	Abi             *abi.ABI
	ContractAddress *common.Address
	SenderAddress   *common.Address
	Bc              *base.BaseBlockChain
	StateDB         *state.StateDB
	PrivateKey      *ecdsa.PrivateKey
}

const PrivateChainCandidateSmcIndex = 5

type CandidateInfo struct {
	Name       string
	Email      string
	Age        *big.Int
	Addr       common.Address
	IsExternal bool
	Source     string
}

func NewCandidateSmcUtil(bc base.BaseBlockChain, key *ecdsa.PrivateKey) (*CandidateSmcUtil, error) {
	stateDb, err := bc.State()
	if err != nil {
		return nil, err
	}
	privateChainSmcAddr, privateChainSmcAbi := configs.GetContractDetailsByIndex(PrivateChainCandidateSmcIndex)
	if privateChainSmcAbi == "" {
		return nil, errors.New("Error getting abi by index")
	}
	abi, err := abi.JSON(strings.NewReader(privateChainSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}
	senderAddr := common.HexToAddress(MockSmartContractCallSenderAccount)
	return &CandidateSmcUtil{Abi: &abi, ContractAddress: &privateChainSmcAddr, SenderAddress: &senderAddr,
		Bc: &bc, StateDB: stateDb, PrivateKey: key}, nil
}

// GetCandidateByEmail returns info of a candidate specified by email, candidate may be from either internal or external
func (cs *CandidateSmcUtil) GetCandidateByEmail(email string) (*CandidateInfo, error) {
	getCandidateInput, err := cs.Abi.Pack("getCandidateInfo", email)
	if err != nil {
		return nil, err
	}
	candidateResult, err := kardia.CallStaticKardiaMasterSmc(*cs.SenderAddress, *cs.ContractAddress, *cs.Bc, getCandidateInput, cs.StateDB)
	if err != nil {
		return nil, err
	}
	var info CandidateInfo
	err = cs.Abi.Unpack(&info, "getCandidateInfo", candidateResult)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// UpdateCandidateInfo returns a tx to call smart contract to add / update an internal candidate of a private chain
func (cs *CandidateSmcUtil) UpdateCandidateInfo(name string, email string, age uint8, address common.Address,
	source string) (*types.Transaction, error) {
	updateCandidateInput, err := cs.Abi.Pack("updateCandidateInfo", name, email, age, address, source)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, updateCandidateInput, state.ManageState(cs.StateDB)), nil
}

// RequestCandidateInfo returns a tx to call smart contract to request info of an candidate from external chain(toOrgId),
// this request will fire an event and captured by Kardia dual node, then answered by corresponding external chain
func (cs *CandidateSmcUtil) RequestCandidateInfo(email string, fromOrgId string, toOrdId string) (*types.Transaction, error) {
	requestCandidateInput, err := cs.Abi.Pack("requestCandidateInfo", email, fromOrgId, toOrdId)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, requestCandidateInput, state.ManageState(cs.StateDB)), nil
}

// UpdateCandidateInfoFromExternal returns a tx to add / update info of a candidate from an external chain
func (cs *CandidateSmcUtil) UpdateCandidateInfoFromExternal(name string, email string, age uint8, address common.Address,
	source string) (*types.Transaction, error) {
	updateExternalCandidateInput, err := cs.Abi.Pack("updateCandidateInfoFromExternal", name, email, age, address, source)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, updateExternalCandidateInput, state.ManageState(cs.StateDB)), nil
}
