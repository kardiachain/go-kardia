package staking

import (
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
func (s *ValidatorSmcUtil) StartValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, valAddr common.Address) error {
	payload, err := s.Abi.Pack("start")
	if err != nil {
		return err
	}
	_, err = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valAddr)
	if err != nil {
		return err
	}

	return nil
}

// Delegate to validator
func (s *ValidatorSmcUtil) Delegate(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, delAddr common.Address, amount *big.Int) error {
	payload, err := s.Abi.Pack("delegate")
	if err != nil {
		return err
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
	if _, err = Apply(s.logger, bc, statedb, header, cfg, msg); err != nil {
		panic(err)
	}

	return nil
}

// GetValidator show info of a validator based on address
func (s *ValidatorSmcUtil) GetInforValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*Validator, error) {
	payload, err := s.Abi.Pack("inforValidator")
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, err
	}

	var validator Validator
	// unpack result
	err = s.Abi.UnpackIntoInterface(&validator, "inforValidator", res)
	if err != nil {
		log.Error("Error unpacking validator info", "err", err)
		return nil, err
	}
	rate, maxRate, maxChangeRate, err := s.GetCommissionValidator(statedb, header, bc, cfg, valSmcAddr)
	if err != nil {
		return nil, err
	}
	validator.CommissionRate = rate
	validator.MaxRate = maxRate
	validator.MaxChangeRate = maxChangeRate
	signingInfo, err := s.GetSigningInfo(statedb, header, bc, cfg, valSmcAddr)
	if err != nil {
		return nil, err
	}
	validator.SigningInfo = signingInfo
	return &validator, nil
}

// GetValidator show info of a validator based on address
func (s *ValidatorSmcUtil) GetCommissionValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*big.Int, *big.Int, *big.Int, error) {
	payload, err := s.Abi.Pack("commission")
	if err != nil {
		return nil, nil, nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	var commission struct {
		Rate          *big.Int
		MaxRate       *big.Int
		MaxChangeRate *big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&commission, "commission", res)
	if err != nil {
		log.Error("Error unpacking validator commission info", "err", err)
		return nil, nil, nil, err
	}
	return commission.Rate, commission.MaxRate, commission.MaxChangeRate, nil
}

// GetDelegators returns all delegators of a validator
func (s *ValidatorSmcUtil) GetDelegators(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) ([]*Delegator, error) {
	payload, err := s.Abi.Pack("getDelegations")
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, err
	}

	var delegations struct {
		DelAddrs []common.Address
		Shares   []*big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegations, "getDelegations", res)
	if err != nil {
		log.Error("Error unpacking delegation details", "err", err)
		return nil, err
	}
	var delegators []*Delegator
	for _, delAddr := range delegations.DelAddrs {
		reward, err := s.GetDelegationRewards(statedb, header, bc, cfg, valSmcAddr, delAddr)
		if err != nil {
			return nil, err
		}
		stakedAmount, err := s.GetDelegatorStakedAmount(statedb, header, bc, cfg, valSmcAddr, delAddr)
		if err != nil {
			return nil, err
		}
		delegators = append(delegators, &Delegator{
			Address:      delAddr,
			StakedAmount: stakedAmount,
			Reward:       reward,
		})
	}
	return delegators, nil
}

// GetDelegationRewards returns reward of a delegation
func (s *ValidatorSmcUtil) GetDelegationRewards(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, delegatorAddr common.Address) (*big.Int, error) {
	payload, err := s.Abi.Pack("getDelegationRewards", delegatorAddr)
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, err
	}

	var rewards struct {
		Rewards *big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&rewards, "getDelegationRewards", res)
	if err != nil {
		log.Error("Error unpacking delegation rewards", "err", err)
		return nil, err
	}
	return rewards.Rewards, nil
}

// GetDelegatorStakedAmount returns staked amount of a delegator to current validator
func (s *ValidatorSmcUtil) GetDelegatorStakedAmount(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address, delegatorAddr common.Address) (*big.Int, error) {
	payload, err := s.Abi.Pack("delegationByAddr", delegatorAddr)
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, err
	}

	var delegation struct {
		Stake          *big.Int
		PreviousPeriod *big.Int
		Height         *big.Int
		Shares         *big.Int
		Owner          common.Address
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegation, "delegationByAddr", res)
	if err != nil {
		log.Error("Error unpacking delegator's staked amount", "err", err)
		return nil, err
	}
	return delegation.Stake, nil
}

// GetSigningInfo returns signing info of this validator
func (s *ValidatorSmcUtil) GetSigningInfo(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*SigningInfo, error) {
	payload, err := s.Abi.Pack("signingInfo")
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, err
	}
	var result SigningInfo
	// unpack result
	err = s.Abi.UnpackIntoInterface(&result, "signingInfo", res)
	if err != nil {
		log.Error("Error unpacking signing info of validator: ", "err", err)
		return nil, err
	}
	return &result, nil
}

func (s *ValidatorSmcUtil) ConstructAndApplySmcCallMsg(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, payload []byte, valSmcAddr common.Address, valAddr common.Address) ([]byte, error) {
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
