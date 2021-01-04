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
	stypes "github.com/kardiachain/go-kardia/mainchain/staking/types"
	"github.com/kardiachain/go-kardia/types"
)

// MaximumGasToCallStaticFunction ...
var (
	MaximumGasToCallStaticFunction = uint(4000000)
	ToExecResult                   = func(err error) *kvm.ExecutionResult {
		return &kvm.ExecutionResult{
			Err: err,
		}
	}
)

// StakingSmcUtil ...
type StakingSmcUtil struct {
	Abi             *abi.ABI
	ContractAddress common.Address
	Bytecode        string
	logger          log.Logger
}

type Validator struct {
	Name                  [32]uint8      `json:"name"`
	ValAddr               common.Address `json:"validatorAddress"`
	ValStakingSmc         common.Address `json:"valStakingSmc"`
	Tokens                *big.Int       `json:"tokens"`
	Jailed                bool           `json:"jailed"`
	DelegationShares      *big.Int       `json:"delegationShares"`
	AccumulatedCommission *big.Int       `json:"accumulatedCommission"`
	UbdEntryCount         *big.Int       `json:"ubdEntryCount"`
	UpdateTime            *big.Int       `json:"updateTime"`
	Status                uint8          `json:"status"`
	UnbondingTime         *big.Int       `json:"unbondingTime"`
	UnbondingHeight       *big.Int       `json:"unbondingHeight"`
	CommissionRate        *big.Int       `json:"commissionRate,omitempty"`
	MaxRate               *big.Int       `json:"maxRate,omitempty"`
	MaxChangeRate         *big.Int       `json:"maxChangeRate,omitempty"`
	SigningInfo           *SigningInfo   `json:"signingInfo"`
	Delegators            []*Delegator   `json:"delegators,omitempty"`
}

type SigningInfo struct {
	StartHeight        *big.Int `json:"startHeight"`
	IndexOffset        *big.Int `json:"indexOffset"`
	Tombstoned         bool     `json:"tombstoned"`
	MissedBlockCounter *big.Int `json:"missedBlockCounter"`
	JailedUntil        *big.Int `json:"jailedUntil"`
}

type Delegator struct {
	Address      common.Address `json:"address"`
	StakedAmount *big.Int       `json:"stakedAmount"`
	Reward       *big.Int       `json:"reward"`
}

// NewSmcStakingUtil ...
func NewSmcStakingUtil() (*StakingSmcUtil, error) {
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
	SenderAddress common.Address) ([]byte, *kvm.ExecutionResult) {

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
	_name string,
	_commission string,
	_maxRate string,
	_maxChangeRate string,
	_selfDelegate string) *kvm.ExecutionResult {

	commission, k1 := big.NewInt(0).SetString(_commission, 10)
	maxRate, k2 := big.NewInt(0).SetString(_maxRate, 10)
	maxChangeRate, k3 := big.NewInt(0).SetString(_maxChangeRate, 10)
	selfDelegate, k4 := big.NewInt(0).SetString(_selfDelegate, 10)

	name := []byte(_name)
	var arrName [32]byte
	copy(arrName[:], name[:32])

	if !k1 || !k2 || !k3 || !k4 {
		panic("Error while parsing genesis validator params")
	}

	input, err := s.Abi.Pack("createValidator",
		arrName,
		commission,    // Commission rate
		maxRate,       // Maximum commission rate
		maxChangeRate, // Maximum commission change rate
	)
	if err != nil {
		panic(err)
	}

	msg := types.NewMessage(
		valAddr,
		&s.ContractAddress,
		0,
		selfDelegate,  // Self delegate amount
		5000000,       // Gas limit
		big.NewInt(1), // Gas price
		input,
		false,
	)
	if result := Apply(s.logger, bc, statedb, header, cfg, msg); result.Failed() {
		reason, unpackErr := result.UnpackRevertReason()
		panic(fmt.Errorf("%v %s %v", result.Unwrap(), reason, unpackErr))
	}

	return ToExecResult(nil)
}

//ApplyAndReturnValidatorSets allow appy and return validator set
func (s *StakingSmcUtil) ApplyAndReturnValidatorSets(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) ([]*types.Validator, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("getValidatorSets")
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if result.Failed() {
		return nil, result
	}
	if len(result.Return()) == 0 {
		return nil, ToExecResult(nil)
	}

	var valSet struct {
		ValAddrs []common.Address
		Powers   []*big.Int
	}

	//unpack result
	err = s.Abi.UnpackIntoInterface(&valSet, "getValidatorSets", result.Return())
	if err != nil {
		log.Error("Error unpacking val set info", "err", err)
		return nil, ToExecResult(err)
	}

	vals := make([]*types.Validator, len(valSet.ValAddrs))
	for i, valAddr := range valSet.ValAddrs {
		vals[i] = types.NewValidator(valAddr, valSet.Powers[i].Int64())
	}
	return vals, ToExecResult(nil)
}

func (s *StakingSmcUtil) ConstructAndApplySmcCallMsg(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, payload []byte) *kvm.ExecutionResult {
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
func (s *StakingSmcUtil) Mint(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) (*big.Int, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("mint")
	if err != nil {
		return nil, ToExecResult(err)
	}

	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if result.Failed() {
		return nil, result
	}
	if len(result.Return()) == 0 {
		return nil, ToExecResult(nil)
	}
	fee := new(struct {
		Fee *big.Int
	})

	if err := s.Abi.UnpackIntoInterface(fee, "mint", result.Return()); err != nil {
		return nil, ToExecResult(fmt.Errorf("unpack mint result err: %s", err))
	}
	statedb.AddBalance(s.ContractAddress, fee.Fee)
	return fee.Fee, ToExecResult(nil)
}

//FinalizeCommit finalize commit
func (s *StakingSmcUtil) FinalizeCommit(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, lastCommit stypes.LastCommitInfo) *kvm.ExecutionResult {
	vals := make([]common.Address, len(lastCommit.Votes))
	votingPowers := make([]*big.Int, len(lastCommit.Votes))
	signed := make([]bool, len(lastCommit.Votes))

	for idx, voteInfo := range lastCommit.Votes {
		vals[idx] = voteInfo.Address
		votingPowers[idx] = voteInfo.VotingPower
		signed[idx] = voteInfo.SignedLastBlock
	}

	payload, err := s.Abi.Pack("finalize", vals, votingPowers, signed)
	if err != nil {
		return ToExecResult(err)
	}
	return s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
}

//DoubleSign double sign
func (s *StakingSmcUtil) DoubleSign(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, byzVals []stypes.Evidence) *kvm.ExecutionResult {
	var result *kvm.ExecutionResult
	for _, ev := range byzVals {
		payload, err := s.Abi.Pack("doubleSign", ev.Address, ev.VotingPower, big.NewInt(int64(ev.Height)))
		if err != nil {
			return ToExecResult(err)
		}
		result = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
		if result.Failed() {
			return result
		}
	}
	return ToExecResult(nil)
}

// GetAllValsLength returns number of validators
func (s *StakingSmcUtil) GetAllValsLength(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) (*big.Int, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("allValsLength")
	if err != nil {
		return nil, ToExecResult(err)
	}

	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if result.Failed() {
		return nil, result
	}
	if len(result.Return()) == 0 {
		return nil, ToExecResult(nil)
	}

	var numberVals *big.Int
	// unpack result
	err = s.Abi.UnpackIntoInterface(&numberVals, "allValsLength", result.Return())
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
		return nil, ToExecResult(err)
	}
	return numberVals, ToExecResult(nil)
}

// GetValFromOwner returns address validator smc of validator
func (s *StakingSmcUtil) GetValFromOwner(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address) (common.Address, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("ownerOf", valAddr)
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if result.Failed() {
		return common.Address{}, result
	}
	var valSmc struct {
		AddrValSmc common.Address
	}
	err = s.Abi.UnpackIntoInterface(&valSmc, "ownerOf", result.Return())
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
	}

	return valSmc.AddrValSmc, ToExecResult(nil)
}

func (s *StakingSmcUtil) GetValSmcAddr(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, index *big.Int) (common.Address, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("allVals", index)
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if result.Failed() {
		return common.Address{}, result
	}
	var valSmc struct {
		AddrValSmc common.Address
	}
	err = s.Abi.UnpackIntoInterface(&valSmc, "allVals", result.Return())
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
	}

	return valSmc.AddrValSmc, ToExecResult(nil)
}

// GetValidatorsByDelegator returns all validators to whom this delegator delegated
func (s *StakingSmcUtil) GetValidatorsByDelegator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, delAddr common.Address) ([]common.Address, *kvm.ExecutionResult) {
	payload, err := s.Abi.Pack("getValidatorsByDelegator", delAddr)
	if err != nil {
		return nil, ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if result.Failed() {
		return nil, result
	}
	var valsAddr struct {
		ValAddrs []common.Address
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&valsAddr, "getValidatorsByDelegator", result.Return())
	if err != nil {
		log.Error("Error unpacking validators by delegator", "err", err)
		return nil, ToExecResult(err)
	}
	return valsAddr.ValAddrs, ToExecResult(nil)
}

// SetRoot set address root
func (s *StakingSmcUtil) SetRoot(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) *kvm.ExecutionResult {
	payload, err := s.Abi.Pack("transferOwnership", s.ContractAddress)
	if err != nil {
		return ToExecResult(err)
	}
	result := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	return result
}

// Apply ...
func Apply(logger log.Logger, bc vm.ChainContext, statedb *state.StateDB, header *types.Header, cfg kvm.Config, msg types.Message) *kvm.ExecutionResult {
	// Create a new context to be used in the EVM environment
	context := vm.NewKVMContext(msg, header, bc)
	vmenv := kvm.NewKVM(context, statedb, cfg)
	sender := kvm.AccountRef(msg.From())
	ret, _, vmerr := vmenv.Call(sender, *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	if vmerr != nil {
		return &kvm.ExecutionResult{
			UsedGas:    0,
			Err:        vmerr,
			ReturnData: ret,
		}
	}
	// Update the state with pending changes
	statedb.Finalise(true)
	return &kvm.ExecutionResult{
		UsedGas:    0,
		Err:        nil,
		ReturnData: ret,
	}
}

// CreateStakingContract ...
func (s *StakingSmcUtil) CreateStakingContract(statedb *state.StateDB,
	header *types.Header,
	cfg kvm.Config) *kvm.ExecutionResult {

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
		return ToExecResult(err)
	}
	// Update the state with pending changes
	statedb.Finalise(true)
	return ToExecResult(nil)
}
