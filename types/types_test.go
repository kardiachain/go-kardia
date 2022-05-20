package types

import (
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

type devnull struct{ len int }

func (d *devnull) Write(p []byte) (int, error) {
	d.len += len(p)
	return len(p), nil
}

func BenchmarkEncodeRLP(b *testing.B) {
	benchRLP(b, true)
}

func BenchmarkDecodeRLP(b *testing.B) {
	benchRLP(b, false)
}

func benchRLP(b *testing.B, encode bool) {
	key, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	to := common.HexToAddress("0x00000000000000000000000000000000deadbeef")
	signer := LatestSignerForChainID(big.NewInt(1337))
	receipts := Receipts{
		&Receipt{
			Status:            ReceiptStatusSuccessful,
			CumulativeGasUsed: 0x888888888,
			Logs:              make([]*Log, 0),
		},
	}
	for _, tc := range []struct {
		name string
		obj  interface{}
	}{
		{
			"blockInfo",
			&BlockInfo{
				GasUsed:  100000,
				Rewards:  new(big.Int).SetUint64(14000000000),
				Receipts: receipts,
				Bloom:    CreateBloom(receipts),
			},
		},
		{
			"receipt-for-storage",
			&ReceiptForStorage{
				Status:            ReceiptStatusSuccessful,
				CumulativeGasUsed: 0x888888888,
				Logs:              make([]*Log, 0),
			},
		},
		{
			"receipt-full",
			receipts[0],
		},
		{
			"transaction",
			MustSignNewTx(signer, NewTransaction(1, to, big.NewInt(1), 1000000, big.NewInt(500), nil), key),
		},
	} {
		if encode {
			b.Run(tc.name, func(b *testing.B) {
				b.ReportAllocs()
				var null = &devnull{}
				for i := 0; i < b.N; i++ {
					rlp.Encode(null, tc.obj)
				}
				b.SetBytes(int64(null.len / b.N))
			})
		} else {
			data, _ := rlp.EncodeToBytes(tc.obj)
			// Test decoding
			b.Run(tc.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if err := rlp.DecodeBytes(data, tc.obj); err != nil {
						b.Fatal(err)
					}
				}
				b.SetBytes(int64(len(data)))
			})
		}
	}
}
