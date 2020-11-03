package staking

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	vm "github.com/kardiachain/go-kardiamain/mainchain/kvm"
	"github.com/kardiachain/go-kardiamain/types"
)

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
	Time             time.Time
	TotalVotingPower uint64
}

// SmcUtil ...
type SmcUtil struct {
	Abi             *abi.ABI
	ContractAddress common.Address
	Bytecode        string
	logger          log.Logger
}

// NewSmcStakingUtil ...
func NewSmcStakingUtil() (*SmcUtil, error) {
	stakingSmcAbi := configs.GetContractABIByAddress(configs.StakingContractAddress.Hex())
	bytecodeStaking := configs.GetContractByteCodeByAddress(configs.StakingContractAddress.Hex())

	stakingAbi, err := abi.JSON(strings.NewReader(stakingSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &SmcUtil{Abi: &stakingAbi, ContractAddress: configs.StakingContractAddress, Bytecode: bytecodeStaking}, nil
}

//CreateValidator create validator
func (s *SmcUtil) CreateGenesisValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address, votingPower int64) error {
	input, err := s.Abi.Pack("createValidator", big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0))
	if err != nil {
		panic(err)
	}

	vp := big.NewInt(votingPower)
	tokens := vp.Mul(vp, big.NewInt(int64(math.Pow10(8))))

	msg := types.NewMessage(
		valAddr,
		&s.ContractAddress,
		0,
		tokens,
		10000000,
		big.NewInt(70000),
		input,
		false,
	)
	if _, err = Apply(s.logger, bc, statedb, header, cfg, msg); err != nil {
		panic(err)
	}

	return nil
}

//ApplyAndReturnValidatorSets allow appy and return validator set
func (s *SmcUtil) ApplyAndReturnValidatorSets(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) ([]*types.Validator, error) {
	payload, err := s.Abi.Pack("applyAndReturnValidatorSets")
	if err != nil {
		return nil, err
	}

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

	res, err := Apply(s.logger, bc, statedb, header, cfg, msg)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var valSet struct {
		ValAddrs []common.Address
		Powers   []*big.Int
	}

	//unpack result
	err = s.Abi.Unpack(&valSet, "applyAndReturnValidatorSets", res)
	if err != nil {
		log.Error("Error unpacking val set info", "err", err)
		return nil, err
	}

	vals := make([]*types.Validator, len(valSet.ValAddrs))
	for i, valAddr := range valSet.ValAddrs {
		vals[i] = types.NewValidator(valAddr, valSet.Powers[i].Int64())
	}
	return vals, nil
}

//Mint new tokens for the previous block. Returns fee collected
func (s *SmcUtil) Mint(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) error {
	payload, err := s.Abi.Pack("mint")
	if err != nil {
		return err
	}

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

	res, err := Apply(s.logger, bc, statedb, header, cfg, msg)
	if err != nil {
		return err
	}

	if len(res) > 0 {
		result := new(struct {
			Fee *big.Int
		})

		if err := s.Abi.Unpack(result, "mint", res); err != nil {
			return fmt.Errorf("unpack mint result err: %s", err)
		}
		statedb.AddBalance(s.ContractAddress, result.Fee)
	}
	return nil
}

//FinalizeCommit finalize commit
func (s *SmcUtil) FinalizeCommit(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, lastCommit LastCommitInfo) error {
	vals := make([]common.Address, len(lastCommit.Votes))
	votingPowers := make([]*big.Int, len(lastCommit.Votes))
	signed := make([]bool, len(lastCommit.Votes))

	for idx, voteInfo := range lastCommit.Votes {
		vals[idx] = voteInfo.Address
		votingPowers[idx] = voteInfo.VotingPower
		signed[idx] = voteInfo.SignedLastBlock
	}

	payload, err := s.Abi.Pack("finalizeCommit", vals, votingPowers, signed)
	if err != nil {
		return err
	}

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

	_, err = Apply(s.logger, bc, statedb, header, cfg, msg)
	return err
}

//DoubleSign double sign
func (s *SmcUtil) DoubleSign(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, byzVals []Evidence) error {
	for _, ev := range byzVals {
		payload, err := s.Abi.Pack("doubleSign", ev.Address, ev.VotingPower, big.NewInt(int64(ev.Height)))
		if err != nil {
			return err
		}

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

		_, err = Apply(s.logger, bc, statedb, header, cfg, msg)
		if err != nil {
			return err
		}

	}

	return nil
}

// SetRoot set address root
func (s *SmcUtil) SetRoot(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) error {
	payload, err := s.Abi.Pack("transferOwnership", s.ContractAddress)
	if err != nil {
		return err
	}

	msg := types.NewMessage(
		s.ContractAddress,
		&s.ContractAddress,
		0,
		big.NewInt(0),
		1000000,
		big.NewInt(0),
		payload,
		false,
	)
	_, err = Apply(s.logger, bc, statedb, header, cfg, msg)
	return err
}

// Apply ...
func Apply(logger log.Logger, bc vm.ChainContext, statedb *state.StateDB, header *types.Header, cfg kvm.Config, msg types.Message) ([]byte, error) {
	// Create a new context to be used in the EVM environment
	context := vm.NewKVMContext(msg, header, bc)
	vmenv := kvm.NewKVM(context, statedb, cfg)
	sender := kvm.AccountRef(msg.From())
	ret, _, vmerr := vmenv.Call(sender, *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	if vmerr != nil {
		return nil, vmerr
	}
	// Update the state with pending changes
	statedb.Finalise(true)
	return ret, nil
}

// CreateStakingContract ...
func (s *SmcUtil) CreateStakingContract(statedb *state.StateDB,
	header *types.Header,
	cfg kvm.Config) error {

	msg := types.NewMessage(
		configs.GenesisDeployerAddr,
		nil,
		0,
		big.NewInt(0),
		100000000,
		big.NewInt(0),
		common.FromHex(s.Bytecode),
		false,
	)

	// Create a new context to be used in the EVM environment
	context := vm.NewKVMContext(msg, header, nil)
	vmenv := kvm.NewKVM(context, statedb, cfg)
	sender := kvm.AccountRef(msg.From())
	if err := vmenv.CreateGenesisContractAddress(sender, msg.Data(), msg.Gas(), msg.Value(), s.ContractAddress); err != nil {
		return err
	}
	log.Info("Created genesis staking smart contract", "Deployer", configs.GenesisDeployerAddr.Hex(), "Address", s.ContractAddress.Hex())
	// Update the state with pending changes
	statedb.Finalise(true)
	return nil
}
