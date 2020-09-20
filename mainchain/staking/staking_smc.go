package staking

import (
	"fmt"
	"math"
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
	Bytecode        string
	logger          log.Logger
}

// NewSmcStakingnUtil ...
func NewSmcStakingnUtil() (*StakingSmcUtil, error) {
	stakingSmcAddr, stakingSmcAbi := configs.GetContractDetailsByIndex(KardiaSatkingSmcIndex)
	bytecodeStaking := configs.GetContractByteCodeByAddress(contractAddress)

	abi, err := abi.JSON(strings.NewReader(stakingSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &StakingSmcUtil{Abi: &abi, ContractAddress: stakingSmcAddr, Bytecode: bytecodeStaking}, nil
}

//SetParams set params
func (s *StakingSmcUtil) SetParams(baseProposerReward int64, bonusProposerReward int64,
	slashFractionDowntime int64, slashFractionDoubleSign int64, unBondingTime int64,
	signedBlockWindow int64, minSignedBlockPerWindow int64,
	SenderAddress common.Address) ([]byte, error) {

	// stateDb, err := s.bc.State()
	// if err != nil {
	// 	return nil, err
	// }

	// store, err := s.Abi.Pack("setParams", big.NewInt(100), big.NewInt(600), big.NewInt(baseProposerReward),
	// 	big.NewInt(bonusProposerReward),
	// 	big.NewInt(slashFractionDowntime), big.NewInt(slashFractionDoubleSign),
	// 	big.NewInt(unBondingTime), big.NewInt(signedBlockWindow),
	// 	big.NewInt(minSignedBlockPerWindow))

	// if err != nil {
	// 	log.Error("Error set params", "err", err)
	// 	return nil, err
	// }

	// _, _, err = sample_kvm.Call(s.ContractAddress, store, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	// if err != nil {
	// 	return nil, err
	// }

	return nil, nil
}

//CreateValidator create validator
func (s *StakingSmcUtil) CreateValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address, votingPower int64) error {
	input, err := s.Abi.Pack("createValidator", big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0))
	if err != nil {
		panic(err)
	}

	tokens := big.NewInt(votingPower)
	tokens = tokens.Mul(tokens, big.NewInt(int64(math.Pow10(6))))

	msg := types.NewMessage(
		valAddr,
		&s.ContractAddress,
		0,
		tokens,
		100000000,
		big.NewInt(0),
		input,
		false,
	)
	_, err = Apply(s.logger, bc, statedb, header, cfg, msg)
	return nil
}

//ApplyAndReturnValidatorSets allow appy and return validator set
func (s *StakingSmcUtil) ApplyAndReturnValidatorSets(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) ([]*types.Validator, error) {
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
func (s *StakingSmcUtil) Mint(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) error {
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

	fee := new(big.Int).SetBytes(res)
	if err != nil {
		return err
	}
	fmt.Println("fee", fee)
	statedb.AddBalance(s.ContractAddress, fee)
	return nil
}

//FinalizeCommit finalize commit
func (s *StakingSmcUtil) FinalizeCommit(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, lastCommit LastCommitInfo) error {
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
func (s *StakingSmcUtil) DoubleSign(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, byzVals []Evidence) error {
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
func (s *StakingSmcUtil) SetRoot(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) error {
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
