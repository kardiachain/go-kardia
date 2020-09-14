package staking

import (
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kvm/sample_kvm"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	// KardiaSatkingSmcIndex ...
	KardiaSatkingSmcIndex = 7
	contractAddress       = "0x00000000000000000000000000000000736D1997"
)

// MaximumGasToCallStaticFunction ...
var MaximumGasToCallStaticFunction = uint(4000000)

// VoteInfo ...
type VoteInfo struct {
	Address         common.Address
	VotingPower     *big.Int
	SignedLastBlock bool
}

// LastCommitInfo ...
type LastCommitInfo struct {
	Votes []VoteInfo
}

// Evidence ...
type Evidence struct {
	Address          common.Address
	VotingPower      *big.Int
	Height           uint64
	Time             uint64
	TotalVotingPower uint64
}

// StakingSmcUtil ...
type StakingSmcUtil struct {
	Abi             *abi.ABI
	ContractAddress common.Address
	bc              base.BaseBlockChain
	bytecode        string
}

// NewSmcStakingnUtil ...
func NewSmcStakingnUtil(bc base.BaseBlockChain) (*StakingSmcUtil, error) {

	stakingSmcAddr, stakingSmcAbi := configs.GetContractDetailsByIndex(KardiaSatkingSmcIndex)
	bytecodeStaking := configs.GetContractByteCodeByAddress(contractAddress)

	abi, err := abi.JSON(strings.NewReader(stakingSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &StakingSmcUtil{Abi: &abi, bc: bc, ContractAddress: stakingSmcAddr, bytecode: bytecodeStaking}, nil
}

//SetParams set params
func (s *StakingSmcUtil) SetParams(baseProposerReward int64, bonusProposerReward int64,
	slashFractionDowntime int64, slashFractionDoubleSign int64, unBondingTime int64,
	signedBlockWindow int64, minSignedBlockPerWindow int64,
	SenderAddress common.Address) ([]byte, error) {

	stateDb, err := s.bc.State()
	if err != nil {
		return nil, err
	}

	store, err := s.Abi.Pack("setParams", big.NewInt(100), big.NewInt(600), big.NewInt(baseProposerReward),
		big.NewInt(bonusProposerReward),
		big.NewInt(slashFractionDowntime), big.NewInt(slashFractionDoubleSign),
		big.NewInt(unBondingTime), big.NewInt(signedBlockWindow),
		big.NewInt(minSignedBlockPerWindow))

	if err != nil {
		log.Error("Error set params", "err", err)
		return nil, err
	}

	_, _, err = sample_kvm.Call(s.ContractAddress, store, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//CreateValidator create validator
func (s *StakingSmcUtil) CreateValidator(commssionRate int64, maxRate int64, maxChangeRate int64, minSelfDelegation int64, SenderAddress common.Address, amount int64) (*big.Int, error) {
	stateDb, err := s.bc.State()
	if err != nil {
		return nil, err
	}

	createValidator, err := s.Abi.Pack("createValidator", big.NewInt(commssionRate), big.NewInt(maxRate), big.NewInt(maxChangeRate), big.NewInt(minSelfDelegation))
	if err != nil {
		return nil, err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, createValidator, &sample_kvm.Config{State: stateDb, Value: big.NewInt(amount), Origin: SenderAddress})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//ApplyAndReturnValidatorSets allow appy and return validator set
func (s *StakingSmcUtil) ApplyAndReturnValidatorSets() ([]*types.Validator, error) {
	stateDb, err := s.bc.State()
	if err != nil {
		return nil, err
	}

	payload, err := s.Abi.Pack("applyAndReturnValidatorSets")
	if err != nil {
		return nil, err
	}
	res, _, err := sample_kvm.Call(s.ContractAddress, payload, &sample_kvm.Config{State: stateDb, Origin: s.ContractAddress})
	if err != nil {
		return nil, err
	}

	var valSet struct {
		ValAddrs []common.Address
		Powers   []*big.Int
	}

	if len(res) == 0 {
		return nil, nil
	}

	//unpack result
	err = s.Abi.Unpack(&valSet, "applyAndReturnValidatorSets", res)
	if err != nil {
		log.Error("Error unpacking val set info", "err", err)
		return nil, err
	}

	vals := make([]*types.Validator, len(valSet.ValAddrs))
	for i, valAddr := range valSet.ValAddrs {
		vals[i] = types.NewValidator(valAddr, valSet.Powers[i].Uint64())
	}

	return vals, nil
}

//Mint new tokens for the previous block. Returns fee collected
func (s *StakingSmcUtil) Mint() error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	mint, err := s.Abi.Pack("mint")
	if err != nil {
		return err
	}
	res, _, err := sample_kvm.Call(s.ContractAddress, mint, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return err
	}

	fee := new(big.Int).SetBytes(res)
	if err != nil {
		return err
	}

	stateDb.AddBalance(s.ContractAddress, fee)

	return nil
}

//FinalizeCommit finalize commit
func (s *StakingSmcUtil) FinalizeCommit(lastCommit LastCommitInfo) error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	vals := make([]common.Address, len(lastCommit.Votes))
	votingPowers := make([]*big.Int, len(lastCommit.Votes))
	signed := make([]bool, len(lastCommit.Votes))

	for idx, voteInfo := range lastCommit.Votes {
		vals[idx] = voteInfo.Address
		votingPowers[idx] = voteInfo.VotingPower
		signed[idx] = voteInfo.SignedLastBlock
	}

	finalizeCommit, err := s.Abi.Pack("finalizeCommit", vals, votingPowers, signed)
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, finalizeCommit, &sample_kvm.Config{State: stateDb, Origin: s.ContractAddress})
	if err != nil {
		return err
	}

	return nil
}

//DoubleSign double sign
func (s *StakingSmcUtil) DoubleSign(byzVals []Evidence) error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	for _, ev := range byzVals {
		payload, err := s.Abi.Pack("doubleSign", ev.Address, ev.VotingPower, big.NewInt(int64(ev.Height)))
		if err != nil {
			return err
		}
		_, _, err = sample_kvm.Call(s.ContractAddress, payload, &sample_kvm.Config{State: stateDb, Origin: s.ContractAddress})
		if err != nil {
			return err
		}

	}

	return nil
}

//GetCurrentValidatorSet get current validator set
func (s *StakingSmcUtil) GetCurrentValidatorSet() ([]*types.Validator, error) {
	return s.ApplyAndReturnValidatorSets()
}

// SetRoot set address root
func (s *StakingSmcUtil) SetRoot(rootAddr common.Address) error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	payload, err := s.Abi.Pack("setRoot", rootAddr)
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, payload, &sample_kvm.Config{State: stateDb})
	return err
}
