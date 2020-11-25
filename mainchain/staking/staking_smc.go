package staking

import (
	"fmt"
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
	Time             time.Time
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
	stakingSmcAbi := configs.GetContractABIByAddress(configs.DefaultStakingContractAddress)
	bytecodeStaking := configs.GetContractByteCodeByAddress(configs.DefaultStakingContractAddress)
	abi, err := abi.JSON(strings.NewReader(stakingSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &StakingSmcUtil{Abi: &abi, ContractAddress: common.HexToAddress(configs.DefaultStakingContractAddress), Bytecode: bytecodeStaking}, nil
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
func (s *StakingSmcUtil) CreateGenesisValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config,
	valAddr common.Address,
	_commission string,
	_maxRate string,
	_maxChangeRate string,
	_minSelfDelegate string,
	_selfDelegate string) error {

	commission, k1 := big.NewInt(0).SetString(_commission, 10)
	maxRate, k2 := big.NewInt(0).SetString(_maxRate, 10)
	maxChangeRate, k3 := big.NewInt(0).SetString(_maxChangeRate, 10)
	minSelfDelegate, k4 := big.NewInt(0).SetString(_minSelfDelegate, 10)
	selfDelegate, k5 := big.NewInt(0).SetString(_selfDelegate, 10)

	if !k1 || !k2 || !k3 || !k4 || !k5 {
		panic("Error while parsing genesis validator params")
	}

	input, err := s.Abi.Pack("createValidator",
		commission,      // Commission rate
		maxRate,         // Maximum commission rate
		maxChangeRate,   // Maximum commission change rate
		minSelfDelegate, // Minimum self delegate amount
	)
	if err != nil {
		panic(err)
	}

	msg := types.NewMessage(
		valAddr,
		&s.ContractAddress,
		0,
		selfDelegate,  // Self delegate amount
		3000000,       // Gas limit
		big.NewInt(1), // Gas price
		input,
		false,
	)
	if _, err = Apply(s.logger, bc, statedb, header, cfg, msg); err != nil {
		panic(err)
	}

	return nil
}

//ApplyAndReturnValidatorSets allow appy and return validator set
func (s *StakingSmcUtil) ApplyAndReturnValidatorSets(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) ([]*types.Validator, error) {
	payload, err := s.Abi.Pack("applyAndReturnValidatorSets")
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
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
	err = s.Abi.UnpackIntoInterface(&valSet, "applyAndReturnValidatorSets", res)
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

// GetValidators show all validators in network
func (s *StakingSmcUtil) GetValidators(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) ([]*types.Validator, error) {
	payload, err := s.Abi.Pack("getValidators")
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var valsInfoList struct {
		ValAddrs          []common.Address
		Tokens            []*big.Int
		DelegationsShares []*big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&valsInfoList, "getValidators", res)
	if err != nil {
		log.Error("Error unpacking validators list info", "err", err)
		return nil, err
	}
	vals := make([]*types.Validator, len(valsInfoList.ValAddrs))
	for i := range valsInfoList.ValAddrs {
		votingPower, err := s.GetValidatorPower(statedb, header, bc, cfg, valsInfoList.ValAddrs[i])
		if err != nil {
			return nil, err
		}
		vals[i] = types.NewValidator(valsInfoList.ValAddrs[i], votingPower)
	}
	return vals, nil
}

// GetValidator show info of a validator based on address
func (s *StakingSmcUtil) GetValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address) (*types.Validator, error) {
	payload, err := s.Abi.Pack("getValidator", valAddr)
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var validator struct {
		Tokens           *big.Int
		DelegationShares *big.Int
		Jailed           bool
		Rate             *big.Int
		MaxRate          *big.Int
		MaxChangeRate    *big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&validator, "getValidator", res)
	if err != nil {
		log.Error("Error unpacking validator info", "err", err)
		return nil, err
	}
	votingPower, err := s.GetValidatorPower(statedb, header, bc, cfg, valAddr)
	if err != nil {
		return nil, err
	}
	val := &types.Validator{
		Address:        valAddr,
		VotingPower:    votingPower,
		StakedAmount:   validator.Tokens,
		CommissionRate: validator.Rate,
		MaxRate:        validator.MaxRate,
		MaxChangeRate:  validator.MaxChangeRate,
	}
	val.StakedAmount = validator.Tokens
	return val, nil
}

// GetValidator returns voting power of a validator based on address
func (s *StakingSmcUtil) GetValidatorPower(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address) (int64, error) {
	payload, err := s.Abi.Pack("getValidatorPower", valAddr)
	if err != nil {
		return 0, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return 0, err
	}
	if len(res) == 0 {
		return 0, nil
	}

	var votingPower *big.Int
	// unpack result
	err = s.Abi.UnpackIntoInterface(&votingPower, "getValidatorPower", res)
	if err != nil {
		log.Error("Error unpacking validator power", "err", err)
		return 0, err
	}
	return votingPower.Int64(), nil
}

// GetValidatorCommission returns commission of a validator based on address
func (s *StakingSmcUtil) GetValidatorCommission(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address) (*big.Int, error) {
	payload, err := s.Abi.Pack("getValidatorCommission", valAddr)
	if err != nil {
		return nil, err
	}

	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var validatorCommission *big.Int
	// unpack result
	err = s.Abi.UnpackIntoInterface(&validatorCommission, "getValidatorCommission", res)
	if err != nil {
		log.Error("Error unpacking validator commission", "err", err)
		return nil, err
	}
	return validatorCommission, nil
}

// GetDelegationsByValidator returns all delegations to a specified validator address
func (s *StakingSmcUtil) GetDelegationsByValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address) ([]*types.Delegator, error) {
	payload, err := s.Abi.Pack("getDelegationsByValidator", valAddr)
	if err != nil {
		return nil, err
	}

	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var delegationsList struct {
		DelAddrs []common.Address
		Shares   []*big.Int
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegationsList, "getDelegationsByValidator", res)
	if err != nil {
		log.Error("Error unpacking delegations list info", "err", err)
		return nil, err
	}
	dels := make([]*types.Delegator, len(delegationsList.DelAddrs))
	for i := range delegationsList.DelAddrs {
		reward, err := s.GetDelegationRewards(statedb, header, bc, cfg, valAddr, delegationsList.DelAddrs[i])
		if err != nil {
			return nil, err
		}
		stake, err := s.GetDelegatorStake(statedb, header, bc, cfg, valAddr, delegationsList.DelAddrs[i])
		if err != nil {
			return nil, err
		}
		dels[i] = &types.Delegator{
			Address:      delegationsList.DelAddrs[i],
			StakedAmount: stake,
			Reward:       reward,
		}
	}
	return dels, nil
}

// GetDelegationRewards returns reward of a delegation
func (s *StakingSmcUtil) GetDelegationRewards(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address, delAddr common.Address) (*big.Int, error) {
	payload, err := s.Abi.Pack("getDelegationRewards", valAddr, delAddr)
	if err != nil {
		return nil, err
	}

	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var delegationReward *big.Int
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegationReward, "getDelegationRewards", res)
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
		return nil, err
	}
	return delegationReward, nil
}

// GetDelegatorStake returns number of token staked by delegator
func (s *StakingSmcUtil) GetDelegatorStake(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address, delAddr common.Address) (*big.Int, error) {
	payload, err := s.Abi.Pack("getDelegatorStake", valAddr, delAddr)
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}

	var delegatorStake *big.Int
	// unpack result
	err = s.Abi.UnpackIntoInterface(&delegatorStake, "getDelegatorStake", res)
	if err != nil {
		log.Error("Error unpacking delegator stake", "err", err)
		return nil, err
	}
	return delegatorStake, nil
}

func (s *StakingSmcUtil) ConstructAndApplySmcCallMsg(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, payload []byte) ([]byte, error) {
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

//Mint new tokens for the previous block. Returns fee collected
func (s *StakingSmcUtil) Mint(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) (*big.Int, error) {
	payload, err := s.Abi.Pack("mint")
	if err != nil {
		return nil, err
	}

	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}
	var fee *big.Int
	if len(res) > 0 {
		result := new(struct {
			Fee *big.Int
		})

		if err := s.Abi.UnpackIntoInterface(result, "mint", res); err != nil {
			return nil, fmt.Errorf("unpack mint result err: %s", err)
		}
		fee = result.Fee
		statedb.AddBalance(s.ContractAddress, fee)
	}
	return fee, nil
}

//FinalizeCommit finalize commitcd
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
	_, err = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	return err
}

//DoubleSign double sign
func (s *StakingSmcUtil) DoubleSign(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, byzVals []Evidence) error {
	for _, ev := range byzVals {
		payload, err := s.Abi.Pack("doubleSign", ev.Address, ev.VotingPower, big.NewInt(int64(ev.Height)))
		if err != nil {
			return err
		}
		_, err = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
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
	_, err = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
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
func (s *StakingSmcUtil) CreateStakingContract(statedb *state.StateDB,
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
	// Update the state with pending changes
	statedb.Finalise(true)
	return nil
}
