package vm

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/kardiachain/go-kardia/lib/common"
)

// precompiledTest defines the input/output pairs for precompiled contract tests.
type precompiledTest struct {
	input, expected string
	gas             uint64
	name            string
	noBenchmark     bool // Benchmark primarily the worst-cases
}

func benchmarkPrecompiled(addr string, test precompiledTest, bench *testing.B) {
	if test.noBenchmark {
		return
	}
	p := PrecompiledContractsV0[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.input)
	reqGas := p.RequiredGas(in)
	contract := NewContract(AccountRef(common.HexToAddress("1337")),
		nil, new(big.Int), reqGas)

	var (
		res  []byte
		err  error
		data = make([]byte, len(in))
	)

	bench.Run(fmt.Sprintf("%s-Gas=%d", test.name, contract.Gas), func(bench *testing.B) {
		bench.ResetTimer()
		for i := 0; i < bench.N; i++ {
			contract.Gas = reqGas
			copy(data, in)
			res, err = RunPrecompiledContract(p, data, contract)
		}
		bench.StopTimer()
		//Check if it is correct
		if err != nil {
			bench.Error(err)
			return
		}
		if common.Bytes2Hex(res) != test.expected {
			bench.Error(fmt.Sprintf("Expected %v, got %v", test.expected, common.Bytes2Hex(res)))
			return
		}
	})
}

// Benchmarks the sample inputs from the ECRECOVER precompile.
func BenchmarkPrecompiledEcrecover(bench *testing.B) {
	t := precompiledTest{
		input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		expected: "000000000000000000000000ceaccac640adf55b2028469bd36ba501f28b699d",
		name:     "",
	}
	benchmarkPrecompiled("01", t, bench)
}

// Benchmarks the sample inputs from the SHA256 precompile.
func BenchmarkPrecompiledSha256(bench *testing.B) {
	t := precompiledTest{
		input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		expected: "811c7003375852fabd0d362e40e68607a12bdabae61a7d068fe5fdd1dbbf2a5d",
		name:     "128",
	}
	benchmarkPrecompiled("02", t, bench)
}

// Benchmarks the sample inputs from the RIPEMD precompile.
func BenchmarkPrecompiledRipeMD(bench *testing.B) {
	t := precompiledTest{
		input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		expected: "0000000000000000000000009215b8d9882ff46f0dfde6684d78e831467f65e6",
		name:     "128",
	}
	benchmarkPrecompiled("03", t, bench)
}

// Benchmarks the sample inputs from the identiy precompile.
func BenchmarkPrecompiledIdentity(bench *testing.B) {
	t := precompiledTest{
		input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		expected: "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		name:     "128",
	}
	benchmarkPrecompiled("04", t, bench)
}
