/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package ethsmc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/kardiachain/go-kardia/lib/abi"
)

// Address of the deployed contract on Rinkeby.
var EthContractAddress = "0x0aa9c07cde3fedcf650473951e77376b1c6a9f16"

// ABI of the deployed Eth contract.
var EthExchangeAbi = `[{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"release","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
{"constant":false,"inputs":[{"name":"receiver","type":"string"},{"name":"destination","type":"string"}],"name":"deposit","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},
{"constant":true,"inputs":[{"name":"destination","type":"string"}],"name":"isValidType","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"},
{"constant":false,"inputs":[{"name":"_type","type":"string"},{"name":"status","type":"bool"}],"name":"updateAvailableType","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
{"inputs":[{"name":"_owner","type":"address"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`

var (
	EthAccountSign     = "0xff6781f2cc6f9b6b4a68a0afc3aae89133bbb236"
	EthAccountSignAddr = "457D86F3AFAA8159D7C8356BF3F195CF7AED35AF84C7DC40C4D9AA27846ED9DC"
)

type EthSmc struct {
	ethABI ethabi.ABI
	kABI   abi.ABI
}

func NewEthSmc() *EthSmc {
	smc := &EthSmc{}
	eABI, err := ethabi.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		panic(fmt.Sprintf("Geth ABI library fail to read abi def: %v", err))
	}
	smc.ethABI = eABI

	kABI, err := abi.JSON(strings.NewReader(EthExchangeAbi))
	if err != nil {
		panic(fmt.Sprintf("Kardia ABI library fail to read abi def: %v", err))
	}
	smc.kABI = kABI

	return smc
}

func (e *EthSmc) etherABI() ethabi.ABI {
	return e.ethABI
}

func (e *EthSmc) InputMethodName(input []byte) (string, error) {
	method, err := e.ethABI.MethodById(input[0:4])
	if err != nil {
		return "", err
	}
	return method.Name, nil
}

// UnpackDepositInput return the receiver address and destination of deposit tx
func (e *EthSmc) UnpackDepositInput(input []byte) (string, string, error) {
	var depositInput struct {
		Receiver    string
		Destination string
	}
	err := e.UnpackInput(&depositInput, "deposit", input[4:])
	if err != nil {
		return "", "", err
	}
	return depositInput.Receiver, depositInput.Destination, nil
}

// UnpackInput unpacks inputs of Method to v according to the abi specification
func (e *EthSmc) UnpackInput(v interface{}, name string, output []byte) (err error) {
	if len(output) == 0 {
		return fmt.Errorf("abi: unmarshalling empty output")
	}
	method, ok := e.ethABI.Methods[name]
	if ok {
		if len(output)%32 != 0 {
			return fmt.Errorf("abi: improperly formatted output")
		}
		return method.Inputs.Unpack(v, output)
	}
	return fmt.Errorf("abi: could not locate named method or event")
}

func (e *EthSmc) packReleaseInput(releaseAddr string, amount *big.Int) []byte {
	address := common.HexToAddress(releaseAddr)
	input, err := e.ethABI.Pack("release", address, amount)
	if err != nil {
		panic(err)
	}
	return input
}

func (e *EthSmc) packDepositInput(receiverAddress string, destinationAddress string) []byte {
	input, err := e.ethABI.Pack("deposit", receiverAddress, destinationAddress)
	if err != nil {
		panic(err)
	}
	return input
}

func (e *EthSmc) CreateEthReleaseTx(amount *big.Int, receiveAddress string, nonce uint64) *types.Transaction {
	contractAddr := common.HexToAddress(EthContractAddress)
	keyBytes, err := hex.DecodeString(EthAccountSignAddr)
	if err != nil {
		panic(err)
	}
	key := crypto.ToECDSAUnsafe(keyBytes)
	data := e.packReleaseInput(receiveAddress, amount)
	gasLimit := uint64(40000)
	gasPrice := big.NewInt(5000000000) // 5gwei
	tx, err := types.SignTx(
		types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, data),
		types.HomesteadSigner{},
		key)
	if err != nil {
		panic(err)
	}

	return tx
}

func (e *EthSmc) CreateEthDepositTx(amount *big.Int, receiver string, destination string, address common.Address, nonce uint64) *types.Transaction {
	keyBytes, err := hex.DecodeString(EthAccountSignAddr)
	if err != nil {
		panic(err)
	}
	key := crypto.ToECDSAUnsafe(keyBytes)
	data := e.packDepositInput(receiver, destination)
	gasLimit := uint64(40000)
	gasPrice := big.NewInt(5000000000) // 5gwei
	tx, err := types.SignTx(
		types.NewTransaction(nonce, address, amount, gasLimit, gasPrice, data),
		types.HomesteadSigner{},
		key)
	if err != nil {
		panic(err)
	}

	return tx
}
