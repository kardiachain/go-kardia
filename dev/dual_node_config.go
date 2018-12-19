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

// Defines default configs used for initializing nodes in dev settings.

package dev

import (
	ethCommon "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/kardiachain/go-kardia/dualnode/eth/ethsmc"
	"math/big"
)

var (
	NeoReceiverAddressList = [2]string{"AVzvbL4pYYK4tfBg8TsZPFcpaozqcSmbWF", "AYKUuUvRSnGJkZFkKRFyXjDWjHFAFQ67p5"}
	EthReceiverAddressList = [2]string{"0x3688Aad7025F17f64eAF8A8De250D3E67f60D9f7", "0x1AbF127Ee9147465Db237Ec986Dc316985e03E3A"}
	OneTenthEthInWei = big.NewInt(0).Exp(big.NewInt(10), big.NewInt(17), nil)
	)
type DualNodeConfig struct {
	Triggering *TriggeringConfig
}

type TriggeringConfig struct {
	// Time interval mimicking how new block is added. For example, [30, 120, 60] indicates that
	// after 30000, 120000, and 60000 mili-seconds a new block is created.
	TimeIntervals []int
	// Whether to repeat TimeIntervals indefinitely after it's exhausted.
	RepeatInfinitely bool

	// Local variables
	nonce uint64
}

func CreateDualNodeConfig() *DualNodeConfig {
	return &DualNodeConfig{
		Triggering: &TriggeringConfig{
			TimeIntervals:    []int{30000},
			RepeatInfinitely: true,
			nonce:            1,
		},
	}
}

func (tc *TriggeringConfig) GenerateEthBlock(address ethCommon.Address) *ethTypes.Block {
	ethSmc := ethsmc.NewEthSmc()
	tx1 := ethSmc.CreateEthDepositTx(OneTenthEthInWei, NeoReceiverAddressList[0], "ETH-NEO", address, tc.nonce)
	return ethTypes.NewBlock(&ethTypes.Header{}, []*ethTypes.Transaction{tx1}, []*ethTypes.Header{}, []*ethTypes.Receipt{})
}
