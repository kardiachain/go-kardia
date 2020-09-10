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

package rand

import (
	"bytes"
	"encoding/json"
	"fmt"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRandStr(t *testing.T) {
	l := 243
	s := Str(l)
	assert.Equal(t, l, len(s))
}

func TestRandBytes(t *testing.T) {
	l := 243
	b := Bytes(l)
	assert.Equal(t, l, len(b))
}

func TestRandIntn(t *testing.T) {
	n := 243
	for i := 0; i < 100; i++ {
		x := Intn(n)
		assert.True(t, x < n)
	}
}

// Test to make sure that we never call math.rand().
// We do this by ensuring that outputs are deterministic.
func TestDeterminism(t *testing.T) {
	var firstOutput string

	// Set math/rand's seed for the sake of debugging this test.
	// (It isn't strictly necessary).
	mrand.Seed(1)

	for i := 0; i < 100; i++ {
		output := testThemAll()
		if i == 0 {
			firstOutput = output
		} else if firstOutput != output {
			t.Errorf("run #%d's output was different from first run.\nfirst: %v\nlast: %v",
				i, firstOutput, output)
		}
	}
}

func testThemAll() string {

	// Such determinism.
	grand.reset(1)

	// Use it.
	out := new(bytes.Buffer)
	perm := Perm(10)
	blob, _ := json.Marshal(perm)
	fmt.Fprintf(out, "perm: %s\n", blob)
	fmt.Fprintf(out, "randInt: %d\n", Int())
	fmt.Fprintf(out, "randUint: %d\n", Uint())
	fmt.Fprintf(out, "randIntn: %d\n", Intn(97))
	fmt.Fprintf(out, "randInt31: %d\n", Int31())
	fmt.Fprintf(out, "randInt32: %d\n", Int32())
	fmt.Fprintf(out, "randInt63: %d\n", Int63())
	fmt.Fprintf(out, "randInt64: %d\n", Int64())
	fmt.Fprintf(out, "randUint32: %d\n", Uint32())
	fmt.Fprintf(out, "randUint64: %d\n", Uint64())
	return out.String()
}

func TestRngConcurrencySafety(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = Uint64()
			<-time.After(time.Millisecond * time.Duration(Intn(100)))
			_ = Perm(3)
		}()
	}
	wg.Wait()
}

func BenchmarkRandBytes10B(b *testing.B) {
	benchmarkRandBytes(b, 10)
}
func BenchmarkRandBytes100B(b *testing.B) {
	benchmarkRandBytes(b, 100)
}
func BenchmarkRandBytes1KiB(b *testing.B) {
	benchmarkRandBytes(b, 1024)
}
func BenchmarkRandBytes10KiB(b *testing.B) {
	benchmarkRandBytes(b, 10*1024)
}
func BenchmarkRandBytes100KiB(b *testing.B) {
	benchmarkRandBytes(b, 100*1024)
}
func BenchmarkRandBytes1MiB(b *testing.B) {
	benchmarkRandBytes(b, 1024*1024)
}

func benchmarkRandBytes(b *testing.B, n int) {
	for i := 0; i < b.N; i++ {
		_ = Bytes(n)
	}
	b.ReportAllocs()
}
