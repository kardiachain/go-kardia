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

package types

import (
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardiamain/lib/common"
)

func TestReceiptEncodeRLP(t *testing.T) {
	// receipt := CreateNewReceipt()
	// receiptCopy := CreateNewReceipt()

	// f, err := os.Create("encodeFile.txt")
	// if err != nil{
	// 	t.Error("Error opening file")
	// }
	// if err := receipt.EncodeRLP(f); err != nil {
	// 	t.Error("Error encoding file")
	// }
	// f.Close()

	// f, err = os.Open("encodeFile.txt")
	// if err != nil{
	// 	t.Error("Error opening file")
	// }
	// stream := rlp.NewStream(f, 99999)

	// if err := receipt.DecodeRLP(stream); err != nil {
	// 	t.Fatal("Error decoding receipt", err)
	// }

	// if rlpHash(receipt) != rlpHash(receiptCopy) {
	// 	t.Error("Error Encoding Decoding Receipt")
	// }
}

func CreateNewReceipt() *Receipt {
	addr := common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	emptyTx := NewTransaction(
		1,
		addr,
		big.NewInt(99), 1000, big.NewInt(100),
		nil,
	)
	return &Receipt{
		PostState:         []byte{0x01},
		Status:            1,
		CumulativeGasUsed: 15,
		Bloom:             [256]byte{0xAA, 0xFF, 0x10},
		Logs:              nil,
		TxHash:            rlpHash(emptyTx),
	}
}
