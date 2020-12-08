package staking

import (
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/types"
)

// MaximumGasToCallStaticFunction ...
// var MaximumGasToCallStaticFunction = uint(4000000)

// StakingSmcUtil ...
type ValidatorSmcUtil struct {
	Abi      *abi.ABI
	Bytecode string
	logger   log.Logger
}

type InforValidator struct {
	Name                  [32]byte
	ValAddr               common.Address
	Tokens                *big.Int
	Jailed                bool
	MinSelfDelegation     *big.Int
	DelegationShares      *big.Int
	AccumulatedCommission *big.Int
	UbdEntryCount         *big.Int
	UpdateTime            *big.Int
	Status                uint8
	UnbondingTime         *big.Int
	UnbondingHeight       *big.Int
}

// NewSmcStakingnUtil ...
func NewSmcValidatorUtil() (*ValidatorSmcUtil, error) {
	validatorSmcAbi := configs.ValidatorContractABI
	bytecodeValidator := configs.ValidatorContractBytecode
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
func (s *ValidatorSmcUtil) GetInforValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valSmcAddr common.Address) (*InforValidator, error) {
	payload, err := s.Abi.Pack("inforValidator")
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload, valSmcAddr, valSmcAddr)
	if err != nil {
		return nil, err
	}

	var validator InforValidator
	// unpack result
	err = s.Abi.UnpackIntoInterface(&validator, "inforValidator", res)
	if err != nil {
		log.Error("Error unpacking validator info", "err", err)
		return nil, err
	}
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
		log.Error("Error unpacking validator info", "err", err)
		return nil, nil, nil, err
	}
	return commission.Rate, commission.MaxRate, commission.MaxChangeRate, nil
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
