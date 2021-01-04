package staking

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/types"
)

// MaximumGasToCallStaticFunction ...
// var MaximumGasToCallStaticFunction = uint(4000000)

// StakingSmcUtil ...
type ValidatorSmcUtil struct {
	Abi      *abi.ABI
	Bytecode string
	logger   log.Logger
}

// NewSmcValidatorUtil
func NewSmcValidatorUtil() (*ValidatorSmcUtil, error) {
	validatorSmcAbi := configs.GetContractABIByType(configs.ValidatorContractKey)
	bytecodeValidator := configs.GetContractByteCodeByType(configs.ValidatorContractKey)
	abi, err := abi.JSON(strings.NewReader(validatorSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &ValidatorSmcUtil{Abi: &abi, Bytecode: bytecodeValidator}, nil
}

//StartValidator start validator
func (s *ValidatorSmcUtil) StartValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, valAddr common.Address) *kvm.ExecutionResult {
	payload, err := s.Abi.Pack("start")
	if err != nil {
		return ToExecResult(err)
	}
	return s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valAddr)
}

// Delegate to validator
func (s *ValidatorSmcUtil) Delegate(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, delAddr common.Address, amount *big.Int) *kvm.ExecutionResult {
	payload, err := s.Abi.Pack("delegate")
	if err != nil {
		return ToExecResult(err)
	}

	msg := types.NewMessage(
		delAddr,
		&valSmcAddr,
		0,
		amount,        // Self delegate amount
		5000000,       // Gas limit
		big.NewInt(1), // Gas price
		payload,
		false,
	)
	if result := Apply(s.logger, bc, statedb, header, cfg, msg); result.Failed() {
		reason, unpackErr := result.UnpackRevertReason()
		panic(fmt.Errorf("%v %s %v", result.Unwrap(), reason, unpackErr))
	}

	return ToExecResult(nil)
}

// GetValidator show info of a validator based on address
func (s *ValidatorSmcUtil) GetInforValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*Validator, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("inforValidator")
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if result.Failed() {
		return nil, result
	}

	var validator Validator
	// unpack result
	err = s.Abi.UnpackIntoInterface(&validator, "inforValidator", result.Return())
	if err != nil {
		log.Error("Error unpacking validator info", "err", err)
		return nil, ToExecResult(err)
	}
	rate, maxRate, maxChangeRate, result := s.GetCommissionValidator(statedb, header, bc, cfg, valSmcAddr)
	if result.Failed() {
		return nil, result
	}
	validator.CommissionRate = rate
	validator.MaxRate = maxRate
	validator.MaxChangeRate = maxChangeRate
	signingInfo, result := s.GetSigningInfo(statedb, header, bc, cfg, valSmcAddr)
	if result.Failed() {
		return nil, result
	}
	validator.SigningInfo = signingInfo
	return &validator, ToExecResult(nil)
}

// GetValidator show info of a validator based on address
func (s *ValidatorSmcUtil) GetCommissionValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*big.Int, *big.Int, *big.Int, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("commission")
	if err != nil {
		return nil, nil, nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if result.Failed() {
		return nil, nil, nil, result
	}

	var commission struct {
		Rate          *big.Int
		MaxRate       *big.Int
		MaxChangeRate *big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&commission, "commission", result.Return())
	if err != nil {
		log.Error("Error unpacking validator commission info", "err", err)
		return nil, nil, nil, ToExecResult(err)
	}
	return commission.Rate, commission.MaxRate, commission.MaxChangeRate, ToExecResult(nil)
}

// GetDelegators returns all delegators of a validator
func (s *ValidatorSmcUtil) GetDelegators(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) ([]*Delegator, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("getDelegations")
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if result.Failed() {
		return nil, result
	}

	var delegations struct {
		DelAddrs []common.Address
		Shares   []*big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegations, "getDelegations", result.Return())
	if err != nil {
		log.Error("Error unpacking delegation details", "err", err)
		return nil, ToExecResult(err)
	}
	var delegators []*Delegator
	for _, delAddr := range delegations.DelAddrs {
		reward, result := s.GetDelegationRewards(statedb, header, bc, cfg, valSmcAddr, delAddr)
		if result.Failed() {
			return nil, ToExecResult(result.Unwrap())
		}
		stakedAmount, result := s.GetDelegatorStakedAmount(statedb, header, bc, cfg, valSmcAddr, delAddr)
		if result.Failed() {
			return nil, ToExecResult(result.Unwrap())
		}
		delegators = append(delegators, &Delegator{
			Address:      delAddr,
			StakedAmount: stakedAmount,
			Reward:       reward,
		})
	}
	return delegators, ToExecResult(nil)
}

// GetDelegationRewards returns reward of a delegation
func (s *ValidatorSmcUtil) GetDelegationRewards(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, delegatorAddr common.Address) (*big.Int, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("getDelegationRewards", delegatorAddr)
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if result.Failed() {
		return nil, result
	}

	var rewards struct {
		Rewards *big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&rewards, "getDelegationRewards", result.Return())
	if err != nil {
		log.Error("Error unpacking delegation rewards", "err", err)
		return nil, ToExecResult(err)
	}
	return rewards.Rewards, ToExecResult(nil)
}

// GetDelegatorStakedAmount returns staked amount of a delegator to current validator
func (s *ValidatorSmcUtil) GetDelegatorStakedAmount(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, delegatorAddr common.Address) (*big.Int, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("getDelegatorStake", delegatorAddr)
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if result.Failed() {
		return nil, result
	}

	var delegation struct {
		DelStake *big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegation, "getDelegatorStake", result.Return())
	if err != nil {
		log.Error("Error unpacking delegator's staked amount", "err", err)
		return nil, ToExecResult(err)
	}
	return delegation.DelStake, ToExecResult(nil)
}

// GetSigningInfo returns signing info of this validator
func (s *ValidatorSmcUtil) GetSigningInfo(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*SigningInfo, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("signingInfo")
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if result.Failed() {
		return nil, result
	}
	var signingInfo SigningInfo
	// unpack result
	err = s.Abi.UnpackIntoInterface(&signingInfo, "signingInfo", result.Return())
	if err != nil {
		log.Error("Error unpacking signing info of validator: ", "err", err)
		return nil, ToExecResult(err)
	}
	return &signingInfo, ToExecResult(nil)
}

func (s *ValidatorSmcUtil) ConstructAndApplySmcCallMsg(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, payload []byte, valSmcAddr common.Address, valAddr common.Address) *kvm.ExecutionResult {
	msg := types.NewMessage(
		valAddr,
		&valSmcAddr,
		0,
		big.NewInt(0),
		100000000,
		big.NewInt(0),
		payload,
		false,
	)
	return Apply(s.logger, bc, statedb, header, cfg, msg)
}
