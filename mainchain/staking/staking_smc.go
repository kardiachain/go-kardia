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
	KardiaSatkingSmcIndex = 7
	contractAddress       = "0x00000000000000000000000000000000736D1997"
)

var MaximumGasToCallStaticFunction = uint(4000000)

type StakingSmcUtil struct {
	Abi             *abi.ABI
	StateDb         *state.StateDB
	ContractAddress common.Address
	bc              base.BaseBlockChain
	bytecode        string
}

func NewSmcStakingnUtil(bc base.BaseBlockChain) (*StakingSmcUtil, error) {
	stateDb, err := bc.State()
	if err != nil {
		log.Error("Error get state", "err", err)
		return nil, err
	}

	stakingSmcAddr, stakingSmcAbi := configs.GetContractDetailsByIndex(KardiaSatkingSmcIndex)
	// fmt.Printf("sasasasasasa %s", stakingSmcAddr)
	// fmt.Printf("stakingSmcAbi %s", stakingSmcAbi)
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

func (s *StakingSmcUtil) SetInflation(number int64, SenderAddress common.Address) ([]byte, error) {
	stateDb := s.StateDb // nen viet 1 lan
	stateDb.SetCode(s.ContractAddress, common.Hex2Bytes(s.bytecode))

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

func (s *StakingSmcUtil) GetInflation(SenderAddress common.Address) (*big.Int, error) {
	stateDb := s.StateDb                                             // nen viet 1 lan
	stateDb.SetCode(s.ContractAddress, common.Hex2Bytes(s.bytecode)) //set contract to stateDB

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

func (s *StakingSmcUtil) SetTotalSupply(number int64, SenderAddress common.Address) ([]byte, error) {
	stateDb := s.StateDb
	stateDb.SetCode(s.ContractAddress, common.Hex2Bytes(s.bytecode))

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

func (s *StakingSmcUtil) GetTotalSupply(SenderAddress common.Address) (*big.Int, error) {
	stateDb := s.StateDb
	stateDb.SetCode(s.ContractAddress, common.Hex2Bytes(s.bytecode))

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

//set parmas
func (s *StakingSmcUtil) SetParams(baseProposerReward int64, bonusProposerReward int64,
	slashFractionDowntime int64, slashFractionDoubleSign int64, unBondingTime int64,
	signedBlockWindow int64, minSignedBlockPerWindow int64,
	SenderAddress common.Address) ([]byte, error) {

	stateDb := s.StateDb
	stateDb.SetCode(s.ContractAddress, common.Hex2Bytes(s.bytecode))

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
		fmt.Printf("err %s", err)
		return nil, err
	}

	return nil, nil
}

//create validator
func (s *StakingSmcUtil) CreateValidator(maxRate int64, maxChangeRate int64, minSelfDelegation int64, SenderAddress common.Address) (*big.Int, error) {

	stateDb := s.StateDb
	stateDb.SetCode(s.ContractAddress, common.Hex2Bytes(s.bytecode))

	// validator1 := common.HexToAddress("0xc1fe56E3F58D3244F606306611a5d10c8333f1f6")
	// maxRate := big.NewInt(20)
	// maxChangeRate := big.NewInt(5)
	// minSelfDelegation := big.NewInt(5)
	createValidator, err := s.Abi.Pack("createValidator", big.NewInt(2), big.NewInt(maxRate), big.NewInt(maxChangeRate), big.NewInt(minSelfDelegation))
	if err != nil {
		return nil, err
	}
	_, _, err = sample_kvm.Call(s.ContractAddress, createValidator, &sample_kvm.Config{State: stateDb, Value: big.NewInt(30), Origin: SenderAddress})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
