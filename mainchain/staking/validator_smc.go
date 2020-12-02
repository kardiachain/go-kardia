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
	Abi             *abi.ABI
	ContractAddress common.Address
	Bytecode        string
	logger          log.Logger
}

// NewSmcStakingnUtil ...
func NewSmcValidatorUtil(valAddr common.Address) (*ValidatorSmcUtil, error) {
	validatorSmcAbi := configs.ValidatorContractABI
	bytecodeValidator := configs.ValidatorContractBytecode
	abi, err := abi.JSON(strings.NewReader(validatorSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &ValidatorSmcUtil{Abi: &abi, ContractAddress: valAddr, Bytecode: bytecodeValidator}, nil
}

//StartValidator start validator
func (s *ValidatorSmcUtil) StartValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) error {
	payload, err := s.Abi.Pack("start")
	if err != nil {
		return err
	}
	_, err = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return err
	}

	return nil
}

//Delegate
func (s *ValidatorSmcUtil) Delegate(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, delAddr common.Address, amount *big.Int) error {
	payload, err := s.Abi.Pack("delegate")
	if err != nil {
		return err
	}

	msg := types.NewMessage(
		delAddr,
		&s.ContractAddress,
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

func (s *ValidatorSmcUtil) ConstructAndApplySmcCallMsg(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, payload []byte) ([]byte, error) {
	msg := types.NewMessage(
		s.ContractAddress,
		&s.ContractAddress,
		0,
		big.NewInt(0),
		100000000,
		big.NewInt(0),
		payload,
		false,
	)
	return Apply(s.logger, bc, statedb, header, cfg, msg)
}
