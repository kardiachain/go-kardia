/*
 *  Copyright 2020 KardiaChain
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

package tests

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
)

func TestGetDelegators(t *testing.T) {
	_, stateDB, stakingUtil, valUtil, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = stakingUtil.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, "Val1", "10", "20", "1", "10")
	if err != nil {
		t.Fatal(err)
	}

	valSmcAddr, err := stakingUtil.GetValSmcAddr(stateDB, block.Header(), nil, kvm.Config{}, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}

	// delegate for this validator
	delAmount, _ := new(big.Int).SetString(selfDelegate, 10)
	err = valUtil.Delegate(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr, address, delAmount)
	if err != nil {
		t.Fatal(err)
	}
	delegators, err := valUtil.GetDelegators(stateDB, block.Header(), nil, kvm.Config{}, valSmcAddr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, address.Hex(), delegators[0].Address.Hex())
	assert.Equal(t, selfDelegate, delegators[0].StakedAmount.String())
}
