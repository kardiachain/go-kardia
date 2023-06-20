/*
 *  Copyright 2021 KardiaChain
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

package tracers

import (
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/mainchain/blockchain"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	vm "github.com/kardiachain/go-kardia/mainchain/kvm"
	"github.com/kardiachain/go-kardia/mainchain/tracers/logger"
	"github.com/kardiachain/go-kardia/tests"
	"github.com/kardiachain/go-kardia/types"
)

// callTrace is the result of a callTracer run.
type callTrace struct {
	Type    string         `json:"type"`
	From    common.Address `json:"from"`
	To      common.Address `json:"to"`
	Input   common.Bytes   `json:"input"`
	Output  common.Bytes   `json:"output"`
	Gas     *common.Uint64 `json:"gas,omitempty"`
	GasUsed *common.Uint64 `json:"gasUsed,omitempty"`
	Value   *common.Big    `json:"value,omitempty"`
	Error   string         `json:"error,omitempty"`
	Calls   []callTrace    `json:"calls,omitempty"`
}

func BenchmarkTransactionTrace(b *testing.B) {
	key, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	from := crypto.PubkeyToAddress(key.PublicKey)
	gas := uint64(1000000) // 1M gas
	to := common.HexToAddress("0x00000000000000000000000000000000deadbeef")
	signer := types.HomesteadSigner{}
	tx, err := types.SignTx(signer, types.NewTransaction(0, to, big.NewInt(0), gas, big.NewInt(500), nil), key)
	if err != nil {
		b.Fatal(err)
	}
	txContext := kvm.TxContext{
		Origin:   from,
		GasPrice: tx.GasPrice(),
	}
	context := kvm.BlockContext{
		CanTransfer: vm.CanTransfer,
		Transfer:    vm.Transfer,
		Coinbase:    common.Address{},
		BlockHeight: new(big.Int).SetUint64(uint64(5)),
		Time:        new(big.Int).SetUint64(uint64(5)),
		GasLimit:    gas,
	}
	alloc := make(map[common.Address]genesis.GenesisAccount)
	// The code pushes 'deadbeef' into memory, then the other params, and calls CREATE2, then returns
	// the address
	loop := []byte{
		byte(kvm.JUMPDEST), //  [ count ]
		byte(kvm.PUSH1), 0, // jumpdestination
		byte(kvm.JUMP),
	}
	alloc[common.HexToAddress("0x00000000000000000000000000000000deadbeef")] = genesis.GenesisAccount{
		Nonce:   1,
		Code:    loop,
		Balance: big.NewInt(1),
	}
	alloc[from] = genesis.GenesisAccount{
		Nonce:   1,
		Code:    []byte{},
		Balance: big.NewInt(500000000000000),
	}
	statedb := tests.MakePreState(rawdb.NewMemoryDatabase().DB(), alloc)
	// Create the tracer, the EVM environment and run it
	tracer := logger.NewStructLogger(&logger.LogConfig{
		Debug: false,
		//DisableStorage: true,
		//EnableMemory: false,
		//EnableReturnData: false,
	})
	evm := kvm.NewKVM(context, txContext, statedb, configs.TestChainConfig, kvm.Config{Debug: true, Tracer: tracer})
	msg, err := tx.AsMessage(signer)
	if err != nil {
		b.Fatalf("failed to prepare transaction for tracing: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		snap := statedb.Snapshot()
		st := blockchain.NewStateTransition(evm, msg, new(types.GasPool).AddGas(tx.Gas()))
		_, err = st.TransitionDb()
		if err != nil {
			b.Fatal(err)
		}
		statedb.RevertToSnapshot(snap)
		if have, want := len(tracer.StructLogs()), 244752; have != want {
			b.Fatalf("trace wrong, want %d steps, have %d", want, have)
		}
		tracer.Reset()
	}
}
