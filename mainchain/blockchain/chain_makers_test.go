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

package blockchain

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/mainchain/staking"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/genesis"
	"github.com/kardiachain/go-kardia/types"
)

func TestGenerateChain(t *testing.T) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		key3, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		addr1   = crypto.PubkeyToAddress(key1.PublicKey)
		addr2   = crypto.PubkeyToAddress(key2.PublicKey)
		addr3   = crypto.PubkeyToAddress(key3.PublicKey)
		db      = storage.NewMemoryDatabase()
		logger  = log.New()
	)

	configs.AddDefaultContract()
	stakingUtil, err := staking.NewSmcStakingUtil()
	if err != nil {
		t.Fatalf("Init staking util failed: %v", err)
	}

	// Ensure that key1 has some funds in the genesis block.
	gspec := &genesis.Genesis{
		Config:   configs.TestChainConfig,
		GasLimit: configs.BlockGasLimit,
		Alloc:    genesis.GenesisAlloc{addr1: {Balance: big.NewInt(1000000)}},
	}
	genesisBlock, err := gspec.Commit(logger, db, stakingUtil)
	if err != nil {
		t.Fatalf("Commit genesis failed: %v", err)
	}

	// This call generates a chain of 5 blocks. The function runs for
	// each block and adds different features to gen based on the
	// block index.
	signer := types.HomesteadSigner{}
	_, _ = GenerateChain(gspec.Config, genesisBlock, db.DB(), 5, func(i int, gen *BlockGen) {
		switch i {
		case 1:
			// In block 1, addr1 sends addr2 some ether.
			tx, _ := types.SignTx(signer, types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(10000), configs.TxGas, nil, nil), key1)
			gen.AddTx(tx)
		case 2:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			tx1, _ := types.SignTx(signer, types.NewTransaction(gen.TxNonce(addr1), addr2, big.NewInt(1000), configs.TxGas, nil, nil), key1)
			tx2, _ := types.SignTx(signer, types.NewTransaction(gen.TxNonce(addr2), addr3, big.NewInt(1000), configs.TxGas, nil, nil), key2)
			gen.AddTx(tx1)
			gen.AddTx(tx2)
		case 3:
			// Block 3 is empty but was mined by addr3.
			gen.SetProposer(addr3)
		}
	})

	// Import the chain. This runs all block validation rules.
	blockchain, _ := NewBlockChain(logger, db, nil)

	state, _ := blockchain.State()
	t.Logf("last block: #%d\n", blockchain.CurrentBlock().Height())
	fmt.Println("balance of addr1:", state.GetBalance(addr1))
	fmt.Println("balance of addr2:", state.GetBalance(addr2))
	fmt.Println("balance of addr3:", state.GetBalance(addr3))
	// Output:
	// last block: #5
	// balance of addr1: 989000
	// balance of addr2: 10000
	// balance of addr3: 19687500000000001000
}
