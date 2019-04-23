package permissioned

import (
	"crypto/ecdsa"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/tool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/pkg/errors"
	"math/big"
	"strings"
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
	senderAddr := common.HexToAddress(configs.KardiaAccountToCallSmc)
	return &CandidateSmcUtil{Abi: &abi, ContractAddress: &privateChainSmcAddr, SenderAddress: &senderAddr,
		Bc: &bc, StateDB: stateDb, PrivateKey: key}, nil
}

// AddRequest returns a tx to add a request to request list of private chain candidate smart contract
func (cs *CandidateSmcUtil) AddRequest(email string, fromOrgID string, state *state.ManagedState) (*types.Transaction, error) {
	addRequestInput, err := cs.Abi.Pack("addRequest", email, fromOrgID)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, addRequestInput, state), nil
}

// AddResponse returns a tx to add an external response for a candidate into private chain candidate smart contract
func (cs *CandidateSmcUtil) AddExternalResponse(email string, content string, fromOrgID string, state *state.ManagedState) (*types.Transaction, error) {
	addRequestInput, err := cs.Abi.Pack("addExternalResponse", email, fromOrgID, content)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, addRequestInput, state), nil
}
