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
var MaximumGasToCallStaticFunction = uint(4000000)

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
	_name string,
	_commission string,
	_maxRate string,
	_maxChangeRate string,
	_selfDelegate string) error {

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
	if _, err = Apply(s.logger, bc, statedb, header, cfg, msg); err != nil {
		panic(err)
	}

	return nil
}

//SetPreviousProposer
func (s *StakingSmcUtil) StartGenesisValidator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, validatorUtil *ValidatorSmcUtil, valSmcAddr common.Address, valAddr common.Address) error {
	err := validatorUtil.StartValidator(statedb, header, bc, cfg, valSmcAddr, valAddr)
	if err != nil {
		return err
	}

	return nil
}

//ApplyAndReturnValidatorSets allow appy and return validator set
func (s *StakingSmcUtil) ApplyAndReturnValidatorSets(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) ([]*types.Validator, error) {
	payload, err := s.Abi.Pack("getValidatorSets")
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
	err = s.Abi.UnpackIntoInterface(&valSet, "getValidatorSets", res)
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
func (s *StakingSmcUtil) FinalizeCommit(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, lastCommit stypes.LastCommitInfo) error {
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
		return err
	}
	_, err = s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	return err
}

//DoubleSign double sign
func (s *StakingSmcUtil) DoubleSign(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, byzVals []stypes.Evidence) error {
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

// GetAllValsLength returns number of validators
func (s *StakingSmcUtil) GetAllValsLength(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config) (*big.Int, error) {
	payload, err := s.Abi.Pack("allValsLength")
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

	var numberVals *big.Int
	// unpack result
	err = s.Abi.UnpackIntoInterface(&numberVals, "allValsLength", res)
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
		return nil, err
	}
	return numberVals, nil
}

// GetValFromOwner returns address validator smc of validator
func (s *StakingSmcUtil) GetValFromOwner(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, valAddr common.Address) (common.Address, error) {
	payload, err := s.Abi.Pack("ownerOf", valAddr)

	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)

	var valSmc struct {
		AddrValSmc common.Address
	}
	err = s.Abi.UnpackIntoInterface(&valSmc, "ownerOf", res)
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
	}

	return valSmc.AddrValSmc, nil
}

func (s *StakingSmcUtil) GetValSmcAddr(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, index *big.Int) (common.Address, error) {
	payload, err := s.Abi.Pack("allVals", index)
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)

	var valSmc struct {
		AddrValSmc common.Address
	}

	err = s.Abi.UnpackIntoInterface(&valSmc, "allVals", res)
	if err != nil {
		log.Error("Error unpacking delegation reward", "err", err)
	}

	return valSmc.AddrValSmc, nil
}

// GetValidatorsByDelegator returns all validators to whom this delegator delegated
func (s *StakingSmcUtil) GetValidatorsByDelegator(statedb *state.StateDB, header *types.Header, bc vm.ChainContext, cfg kvm.Config, delAddr common.Address) ([]common.Address, error) {
	payload, err := s.Abi.Pack("getValidatorsByDelegator", delAddr)
	if err != nil {
		return nil, err
	}
	res, err := s.ConstructAndApplySmcCallMsg(statedb, header, bc, cfg, payload)
	if err != nil {
		return nil, err
	}

	var valsAddr struct {
		ValAddrs []common.Address
	}
	// unpack result
	err = s.Abi.UnpackIntoInterface(&valsAddr, "getValidatorsByDelegator", res)
	if err != nil {
		log.Error("Error unpacking validators by delegator", "err", err)
		return nil, err
	}
	return valsAddr.ValAddrs, nil
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
	// Create a new context to be used in the KVM environment
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

	// Create a new context to be used in the KVM environment
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
