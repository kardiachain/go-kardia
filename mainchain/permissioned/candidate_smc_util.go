/*
 *  Copyright 2019 KardiaChain
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
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/tool"
	"github.com/kardiachain/go-kardiamain/types"
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
func (cs *CandidateSmcUtil) AddRequest(email string, fromOrgID string, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	addRequestInput, err := cs.Abi.Pack("addRequest", email, fromOrgID)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, addRequestInput, txPool, false), nil
}

// AddResponse returns a tx to add an external response for a candidate into private chain candidate smart contract
func (cs *CandidateSmcUtil) AddExternalResponse(email string, content string, fromOrgID string, txPool *tx_pool.TxPool) (*types.Transaction, error) {
	addRequestInput, err := cs.Abi.Pack("addExternalResponse", email, fromOrgID, content)
	if err != nil {
		return nil, err
	}
	return tool.GenerateSmcCall(cs.PrivateKey, *cs.ContractAddress, addRequestInput, txPool, false), nil
}
