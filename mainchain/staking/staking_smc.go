package staking

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kvm/sample_kvm"
	"github.com/kardiachain/go-kardiamain/lib/abi"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
)

const (
	// KardiaSatkingSmcIndex ...
	KardiaSatkingSmcIndex = 7
	contractAddress       = "0x00000000000000000000000000000000736D1997"
)

// MaximumGasToCallStaticFunction ...
var MaximumGasToCallStaticFunction = uint(4000000)

// StakingSmcUtil ...
type StakingSmcUtil struct {
	Abi             *abi.ABI
	StateDb         *state.StateDB
	ContractAddress common.Address
	bc              base.BaseBlockChain
	bytecode        string
}

// NewSmcStakingnUtil ...
func NewSmcStakingnUtil(bc base.BaseBlockChain) (*StakingSmcUtil, error) {
	stateDb, err := bc.State()
	if err != nil {
		log.Error("Error get state", "err", err)
		return nil, err
	}

	stakingSmcAddr, stakingSmcAbi := configs.GetContractDetailsByIndex(KardiaSatkingSmcIndex)

	bytecodeStaking := configs.GetContractByteCodeByAddress(contractAddress)
	if stakingSmcAbi == "" {
		log.Error("Error getting abi by index")
		return nil, err
	}

	abi, err := abi.JSON(strings.NewReader(stakingSmcAbi))
	if err != nil {
		log.Error("Error reading abi", "err", err)
		return nil, err
	}

	return &StakingSmcUtil{Abi: &abi, StateDb: stateDb, ContractAddress: stakingSmcAddr, bytecode: bytecodeStaking}, nil
}

// SetInflation set inflation
func (s *StakingSmcUtil) SetInflation(number int64, SenderAddress common.Address) ([]byte, error) {
	stateDb := s.StateDb // nen viet 1 lan
	store, err := s.Abi.Pack("setInflation", big.NewInt(number))
	if err != nil {
		log.Error("Error set inflation", "err", err)
		return nil, err
	}

	_, _, err = sample_kvm.Call(s.ContractAddress, store, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		fmt.Printf("err %s", err)
		return nil, err
	}

	return nil, nil
}

// GetInflation get inflation
func (s *StakingSmcUtil) GetInflation(SenderAddress common.Address) (*big.Int, error) {
	stateDb := s.StateDb
	get, err := s.Abi.Pack("getInflation")
	if err != nil {
		log.Error("Error get inflation", "err", err)
		return nil, err
	}

	result, _, err := sample_kvm.Call(s.ContractAddress, get, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	num := new(big.Int).SetBytes(result)
	if err != nil {
		return nil, err
	}

	return num, nil
}

// SetTotalSupply set total supply
func (s *StakingSmcUtil) SetTotalSupply(number int64, SenderAddress common.Address) ([]byte, error) {
	stateDb := s.StateDb
	store, err := s.Abi.Pack("setTotalSupply", big.NewInt(number))
	if err != nil {
		log.Error("Error set total supply", "err", err)
		return nil, err
	}

	_, _, err = sample_kvm.Call(s.ContractAddress, store, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		fmt.Printf("err %s", err)
		return nil, err
	}

	return nil, nil
}

// GetTotalSupply get total supply
func (s *StakingSmcUtil) GetTotalSupply(SenderAddress common.Address) (*big.Int, error) {
	stateDb := s.StateDb
	get, err := s.Abi.Pack("getTotalSupply")
	if err != nil {
		log.Error("Error get total supply", "err", err)
		return nil, err
	}

	result, _, err := sample_kvm.Call(s.ContractAddress, get, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	num := new(big.Int).SetBytes(result)
	if err != nil {
		return nil, err
	}

	return num, nil
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
func (s *StakingSmcUtil) ApplyAndReturnValidatorSets(SenderAddress common.Address) error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	applyAndReturnValidatorSets, err := s.Abi.Pack("applyAndReturnValidatorSets")
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, applyAndReturnValidatorSets, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//GetValidatorSets allow get validator set
func (s *StakingSmcUtil) GetValidatorSets() ([]common.Address, []*big.Int, error) {
	stateDb := s.StateDb

	getValidatorSets, err := s.Abi.Pack("getValidatorSets")
	if err != nil {
		return nil, nil, err
	}
	resultGet, _, err := sample_kvm.Call(s.ContractAddress, getValidatorSets, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, nil, err
	}

	var validatorSet struct {
		ValAddrs []common.Address
		Powers   []*big.Int
	}

	//unpack result
	err = s.Abi.Unpack(&validatorSet, "getValidatorSets", resultGet)
	if err != nil {
		log.Error("Error unpacking node info", "err", err)
	}

	return validatorSet.ValAddrs, validatorSet.Powers, nil
}

//GetValidator allow get validator
func (s *StakingSmcUtil) GetValidator(valAddress common.Address) (*big.Int, *big.Int, bool, error) {
	stateDb := s.StateDb

	getValidator, err := s.Abi.Pack("getValidator", valAddress)
	if err != nil {
		return nil, nil, false, err
	}
	resultGet, _, err := sample_kvm.Call(s.ContractAddress, getValidator, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, nil, false, err
	}

	var validatorInfor struct {
		Tokens           *big.Int
		DelegationShares *big.Int
		Jailed           bool
	}

	//unpack result
	err = s.Abi.Unpack(&validatorInfor, "getValidator", resultGet)
	if err != nil {
		log.Error("Error unpacking node info", "err", err)
	}

	return validatorInfor.Tokens, validatorInfor.DelegationShares, validatorInfor.Jailed, nil
}

//Mint new tokens for the previous block. Returns fee collected
func (s *StakingSmcUtil) Mint() (*big.Int, error) {
	stateDb, err := s.bc.State()
	if err != nil {
		return nil, err
	}

	mint, err := s.Abi.Pack("mint")
	if err != nil {
		return nil, err
	}
	resultGet, _, err := sample_kvm.Call(s.ContractAddress, mint, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, err
	}

	num := new(big.Int).SetBytes(resultGet)
	if err != nil {
		return nil, err
	}

	return num, nil
}

//SetTotalBonded  set total bonded
func (s *StakingSmcUtil) SetTotalBonded(totalBond int64, SenderAddress common.Address) error {
	stateDb := s.StateDb
	setTotalBonded, err := s.Abi.Pack("setTotalBonded", big.NewInt(totalBond))
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, setTotalBonded, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//SetAnnualProvision allow et annual provisoin
func (s *StakingSmcUtil) SetAnnualProvision(annualProvision int64, SenderAddress common.Address) error {
	stateDb := s.StateDb

	setAnnualProvision, err := s.Abi.Pack("setAnnualProvision", big.NewInt(annualProvision))
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, setAnnualProvision, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//GetBlockProvision get block provision
func (s *StakingSmcUtil) GetBlockProvision() (*big.Int, error) {
	stateDb := s.StateDb

	getBlockProvision, err := s.Abi.Pack("getBlockProvision")
	if err != nil {
		return nil, err
	}
	resultGet, _, err := sample_kvm.Call(s.ContractAddress, getBlockProvision, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, err
	}

	num := new(big.Int).SetBytes(resultGet)
	if err != nil {
		return nil, err
	}

	return num, nil
}

//SetMintParams set mint params
func (s *StakingSmcUtil) SetMintParams(inflationRateChange int64, goalBonded int64, blocksPerYear int64,
	inflationMax int64, inflationMin int64, SenderAddress common.Address) error {

	stateDb := s.StateDb

	setMintParams, err := s.Abi.Pack("setMintParams", big.NewInt(inflationRateChange), big.NewInt(goalBonded), big.NewInt(blocksPerYear),
		big.NewInt(inflationMax), big.NewInt(inflationMin))
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, setMintParams, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//SetPreviousProposer allow set previous proposer
func (s *StakingSmcUtil) SetPreviousProposer(previousProposer common.Address, SenderAddress common.Address) error {
	stateDb := s.StateDb

	setPreviousProposer, err := s.Abi.Pack("setPreviousProposer", previousProposer)
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, setPreviousProposer, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//FinalizeCommit finalize commit
func (s *StakingSmcUtil) FinalizeCommit(address []common.Address, powers []*big.Int, signed []bool, SenderAddress common.Address) error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	finalizeCommit, err := s.Abi.Pack("finalizeCommit", address, powers, signed)
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, finalizeCommit, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//GetMissedBlock allow get missed block of validator
func (s *StakingSmcUtil) GetMissedBlock(valAddress common.Address) ([]bool, error) {
	stateDb := s.StateDb

	getMissedBlock, err := s.Abi.Pack("getMissedBlock", valAddress)
	if err != nil {
		return nil, err
	}
	resultGet, _, err := sample_kvm.Call(s.ContractAddress, getMissedBlock, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, err
	}

	var missed struct {
		MissedBlock []bool
	}

	//unpack result
	err = s.Abi.Unpack(&missed, "getMissedBlock", resultGet)
	if err != nil {
		log.Error("Error unpacking", "err", err)
	}

	return missed.MissedBlock, nil
}

//GetDelegationRewards allow get delegation rewards
func (s *StakingSmcUtil) GetDelegationRewards(valAddress common.Address, delAddress common.Address) (*big.Int, error) {
	stateDb := s.StateDb

	getDelegationRewards, err := s.Abi.Pack("getDelegationRewards", valAddress, delAddress)
	if err != nil {
		return nil, err
	}
	resultGet, _, err := sample_kvm.Call(s.ContractAddress, getDelegationRewards, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, err
	}

	num := new(big.Int).SetBytes(resultGet)
	if err != nil {
		return nil, err
	}

	return num, nil
}

//DoubleSign double sign
func (s *StakingSmcUtil) DoubleSign(valAddress common.Address, votingPower int64, distributionHeight int64, SenderAddress common.Address) error {
	stateDb, err := s.bc.State()
	if err != nil {
		return err
	}

	doubleSign, err := s.Abi.Pack("doubleSign", valAddress, big.NewInt(votingPower), big.NewInt(distributionHeight))
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, doubleSign, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//Withdraw Withdraw
func (s *StakingSmcUtil) Withdraw(valAddress common.Address, SenderAddress common.Address) error {
	stateDb := s.StateDb

	doubleSign, err := s.Abi.Pack("withdraw", valAddress)
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, doubleSign, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//Undelegate undelegate
func (s *StakingSmcUtil) Undelegate(valAddress common.Address, amount int64, SenderAddress common.Address) error {
	stateDb := s.StateDb

	undelegate, err := s.Abi.Pack("undelegate", valAddress, big.NewInt(amount))
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, undelegate, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//Unjail Unjail
func (s *StakingSmcUtil) Unjail(SenderAddress common.Address) error {
	stateDb := s.StateDb
	unjail, err := s.Abi.Pack("unjail")
	if err != nil {
		return err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, unjail, &sample_kvm.Config{State: stateDb, Origin: SenderAddress})
	if err != nil {
		return err
	}

	return nil
}

//GetCurrentValidatorSet get current validator set
func (s *StakingSmcUtil) GetCurrentValidatorSet() ([]common.Address, []*big.Int, error) {
	stateDb, err := s.bc.State()
	if err != nil {
		return nil, nil, err
	}

	payload, err := s.Abi.Pack("getCurrentValidatorSet")
	if err != nil {
		return nil, nil, err
	}
	res, _, err := sample_kvm.Call(s.ContractAddress, payload, &sample_kvm.Config{State: stateDb})
	if err != nil {
		return nil, nil, err
	}

	var valSet struct {
		ValAddrs []common.Address
		Powers   []*big.Int
	}

	//unpack result
	err = s.Abi.Unpack(&valSet, "getValidatorSets", res)
	if err != nil {
		log.Error("Error unpacking node info", "err", err)
		return nil, nil, err
	}

	return valSet.ValAddrs, valSet.Powers, nil
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
