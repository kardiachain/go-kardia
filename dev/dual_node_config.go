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
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/kardiachain/go-kardia/dual/ethsmc"
)

const (
	dafaultEthTxAmount = 69
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
			RepeatInfinitely: false,
			nonce:            1,
		},
	}
}

func (tc *TriggeringConfig) GenerateEthBlock(address ethCommon.Address) *ethTypes.Block {
	ethSmc := ethsmc.NewEthSmc()
	tx := ethSmc.CreateEthDepositTx(big.NewInt(dafaultEthTxAmount), address, tc.nonce)
	tc.nonce++

	return ethTypes.NewBlock(&ethTypes.Header{}, []*ethTypes.Transaction{tx}, []*ethTypes.Header{&ethTypes.Header{}}, []*ethTypes.Receipt{&ethTypes.Receipt{}})
}
